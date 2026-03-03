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
func (c *SandboxClient) ExecuteTool(ctx context.Context, toolName string, payloadJSON string) (string, error) {
	req := &ipc.ToolRequest{
		ToolName:    toolName,
		PayloadJson: payloadJSON,
	}

	res, err := c.client.ExecuteTool(ctx, req)
	if err != nil {
		return "", fmt.Errorf("rpc failure bridging to rust sandbox: %w", err)
	}

	if !res.Success {
		return "", fmt.Errorf("sandbox rejected execution: %s", res.ErrorMessage)
	}

	return res.OutputJson, nil
}
