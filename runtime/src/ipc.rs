use tonic::{transport::Server, Request, Response, Status};

pub mod proto {
    tonic::include_proto!("aether.ipc.v1");
}

use proto::sandbox_server::{Sandbox, SandboxServer};
use proto::{ToolRequest, ToolResponse};
use std::os::unix::net::UnixListener;
use std::path::Path;
use tokio_stream::wrappers::UnixListenerStream;

#[derive(Debug, Default)]
pub struct SandboxService {}

#[tonic::async_trait]
impl Sandbox for SandboxService {
    async fn execute_tool(
        &self,
        request: Request<ToolRequest>,
    ) -> Result<Response<ToolResponse>, Status> {
        let req = request.into_inner();
        
        // At this layer, the Rust engine would cross-reference req.tool_name against 
        // the capabilities manifest. For Day 10, we simply establish the IPC chain.
        println!(
            r#"{{"level":"INFO","msg":"tool_received_via_ipc","tool":"{}"}}"#,
            req.tool_name
        );

        let res = ToolResponse {
            success: true,
            output_json: format!(
                r#"{{"sandbox_executed":true,"received_payload":{}}}"#,
                req.payload_json
            ),
            error_message: String::new(),
        };

        Ok(Response::new(res))
    }
}

pub async fn start_uds_server<P: AsRef<Path>>(path: P) -> Result<(), Box<dyn std::error::Error>> {
    let p = path.as_ref();
    if p.exists() {
        std::fs::remove_file(p)?;
    }

    let uds = UnixListener::bind(p)?;
    let stream = UnixListenerStream::new(tokio::net::UnixListener::from_std(uds)?);

    let service = SandboxService::default();

    println!(
        r#"{{"level":"INFO","msg":"sandbox_grpc_listening","socket":"{:?}"}}"#,
        p
    );

    Server::builder()
        .add_service(SandboxServer::new(service))
        .serve_with_incoming(stream)
        .await?;

    Ok(())
}
