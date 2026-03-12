package core

import (
	"context"
	"testing"
)

// TestSandboxClient_ExecuteTool_NoConnection verifies that calling ExecuteTool
// on an uninitialised client (nil gRPC connection) returns an error rather than
// panicking. Signature verification is enforced server-side by the Rust sandbox
// via Ed25519; the Go client no longer duplicates that check.
func TestSandboxClient_ExecuteTool_NoConnection(t *testing.T) {
	client := &SandboxClient{} // conn and client are both nil
	_, err := client.ExecuteTool(context.Background(), "test", "{}", "")
	if err == nil {
		t.Error("expected an error when the gRPC connection is not established, got nil")
	}
}
