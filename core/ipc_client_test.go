package core

import (
	"context"
	"testing"
)

func TestSandboxClient_ExecuteTool_UnsignedRejection(t *testing.T) {
	client := &SandboxClient{}
	_, err := client.ExecuteTool(context.Background(), "test", "{}", "")
	if err == nil || err.Error() != "refusing to dispatch unsigned tool via IPC" {
		t.Errorf("Expected explicit unsigned rejection from IPC bounds")
	}
}
