use serde::Deserialize;
use std::fs;
use std::path::Path;

#[derive(Debug, Deserialize)]
pub struct Manifest {
    pub sandbox: SandboxConfig,
    pub tools: Vec<ToolConfig>,
}

#[derive(Debug, Deserialize)]
pub struct SandboxConfig {
    pub strict_mode: bool,
}

#[derive(Debug, Deserialize)]
pub struct ToolConfig {
    pub name: String,
    pub version: String,
    pub description: String,
    pub capabilities: Vec<String>,
    pub max_runtime_ms: u64,
    pub memory_limit_mb: u64,
}

impl Manifest {
    pub fn load<P: AsRef<Path>>(path: P) -> Result<Self, Box<dyn std::error::Error>> {
        let content = fs::read_to_string(path)?;
        let manifest: Manifest = toml::from_str(&content)?;
        Ok(manifest)
    }
}
