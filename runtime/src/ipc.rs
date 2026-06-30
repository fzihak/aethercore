use tonic::{transport::Server, Request, Response, Status};

pub mod proto {
    tonic::include_proto!("aether.ipc.v1");
}

use proto::sandbox_server::{Sandbox, SandboxServer};
use proto::{ToolRequest, ToolResponse};
use std::os::unix::net::UnixListener;
use std::path::Path;
use tokio_stream::wrappers::UnixListenerStream;

use crate::manifest::Manifest;
use crate::sandbox::CgroupGuard;
use crate::wasm_engine::WasmSandbox;
use ed25519_dalek::VerifyingKey as PublicKey;
use std::sync::Arc;

#[derive(Clone)]
pub struct SandboxService {
    pub manifest: Arc<Manifest>,
    pub pubkey: PublicKey,
    pub wasm_engine: Arc<WasmSandbox>,
}

#[tonic::async_trait]
impl Sandbox for SandboxService {
    async fn execute_tool(
        &self,
        request: Request<ToolRequest>,
    ) -> Result<Response<ToolResponse>, Status> {
        let req = request.into_inner();

        // 1. Locate tool in manifest
        let tool = self.manifest
            .tools
            .iter()
            .find(|t| t.name == req.tool_name)
            .ok_or_else(|| Status::not_found(format!("tool {} not registered in manifest", req.tool_name)))?;

        // 2. Cryptographic Verification
        if let Err(e) = tool.verify(&self.pubkey) {
            eprintln!(
                r#"{{"level":"ERROR","msg":"manifest_verification_failed","tool":"{}","error":"{:?}"}}"#,
                req.tool_name, e
            );
            return Err(Status::permission_denied("manifest_verification_failed"));
        }

        // 3. Apply enforcement cgroups per request
        let memory_limit_bytes = tool.capabilities.max_memory_mb * 1024 * 1024;
        let _guard = CgroupGuard::apply(&tool.name, memory_limit_bytes).map_err(|e| {
            Status::internal(format!("cgroup_apply_failed: {}", e))
        })?;

        // 4. Execute inside WASM (currently without WASI for Day 10 stableness, assuming basic fuel)
        let output = self.wasm_engine.execute(&[], &req.payload_json, &tool.capabilities).unwrap_or_else(|e| {
            format!(r#"{{"error": "{}"}}"#, e)
        });

        let res = ToolResponse {
            success: true,
            output_json: output,
            error_message: String::new(),
        };

        Ok(Response::new(res))
    }
}

pub async fn start_uds_server<P: AsRef<Path>>(
    path: P,
    manifest: Arc<Manifest>,
    pubkey: PublicKey,
    wasm_engine: Arc<WasmSandbox>,
) -> Result<(), Box<dyn std::error::Error>> {
    let p = path.as_ref();
    if p.exists() {
        std::fs::remove_file(p)?;
    }

    // Set process umask to 0o177 to ensure the socket is created with 0o600 permissions
    // This prevents a TOCTOU race condition where an attacker connects before permissions are applied
    struct UmaskGuard(libc::mode_t);
    impl UmaskGuard {
        fn new(mask: libc::mode_t) -> Self {
            Self(unsafe { libc::umask(mask) })
        }
    }
    impl Drop for UmaskGuard {
        fn drop(&mut self) {
            unsafe { libc::umask(self.0) };
        }
    }

    let uds = {
        let _guard = UmaskGuard::new(0o177);
        UnixListener::bind(p)
    };

    let uds = uds?;

    let expected_uid = unsafe { libc::geteuid() };

    use tokio_stream::StreamExt;
    let stream = UnixListenerStream::new(tokio::net::UnixListener::from_std(uds)?);
    let filtered_stream = stream.filter_map(move |item| {
        match item {
            Ok(stream) => {
                match stream.peer_cred() {
                    Ok(cred) if cred.uid() == expected_uid => Some(Ok(stream)),
                    Ok(cred) => {
                        eprintln!(
                            r#"{{"level":"ERROR","msg":"peer_uid_rejected","expected":{},"actual":{}}}"#,
                            expected_uid, cred.uid()
                        );
                        None
                    }
                    Err(e) => {
                        eprintln!(
                            r#"{{"level":"ERROR","msg":"peer_cred_error","error":"{}"}}"#,
                            e
                        );
                        None
                    }
                }
            }
            Err(e) => Some(Err(e)),
        }
    });

    let service = SandboxService {
        manifest,
        pubkey,
        wasm_engine,
    };

    println!(
        r#"{{"level":"INFO","msg":"sandbox_grpc_listening","socket":"{:?}"}}"#,
        p
    );

    Server::builder()
        .add_service(SandboxServer::new(service))
        .serve_with_incoming(filtered_stream)
        .await?;

    Ok(())
}
