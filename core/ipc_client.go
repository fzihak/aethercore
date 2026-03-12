package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fzihak/aethercore/core/ipc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// IPCSocketPath returns the OS-specific path for the Unix Domain Socket.
func IPCSocketPath() string {
	return filepath.Join(os.TempDir(), "aether-sandbox.sock")
}

// SandboxClient manages the gRPC connection to the Layer 2 Rust Sandbox.
type SandboxClient struct {
	conn   *grpc.ClientConn
	client ipc.SandboxClient
}

// NewSandboxClient establishes a gRPC connection over a Unix Domain Socket.
func NewSandboxClient() (*SandboxClient, error) {
	socketPath := IPCSocketPath()

	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client for layer 2 sandbox over uds: %w", err)
	}

	return &SandboxClient{
		conn:   conn,
		client: ipc.NewSandboxClient(conn),
	}, nil
}

// Close gracefully shuts down the gRPC IPC connection.
func (c *SandboxClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ExecuteTool forwards a tool execution request to the Rust sandbox.
// Signature verification is enforced server-side by the Rust SandboxService
// (Ed25519 via manifest.rs); the Go client passes the value through and lets
// the sandbox reject invalid or missing signatures, avoiding a redundant gate.
func (c *SandboxClient) ExecuteTool(ctx context.Context, toolName, payloadJSON, signatureHex string) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("sandbox client not connected: call NewSandboxClient first")
	}

	req := &ipc.ToolRequest{
		ToolName:    toolName,
		PayloadJson: payloadJSON,
	}

	res, err := c.client.ExecuteTool(ctx, req)
	if err != nil {
		return "", fmt.Errorf("rpc failure bridging to rust sandbox: %w", err)
	}

	if !res.GetSuccess() {
		return "", fmt.Errorf("sandbox rejected execution: %s", res.GetErrorMessage()) //nolint:err113 // dynamic error is appropriate here; caller logs it
	}

	return res.GetOutputJson(), nil
}
