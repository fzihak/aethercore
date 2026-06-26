package tools

import (
	"context"
	"testing"

	"github.com/fzihak/aethercore/core/llm"
)

func TestSysInfoTool(t *testing.T) {
	tool := &SysInfoTool{}

	manifest := tool.Manifest()
	if manifest.Name != "sys_info" {
		t.Fatalf("Expected tool name 'sys_info', got '%s'", manifest.Name)
	}

	// Ensure capabilities are strictly constrained
	if len(manifest.Capabilities) != 1 || manifest.Capabilities[0] != llm.CapState {
		t.Fatal("SysInfo tool should strictly only require CapState")
	}

	out, err := tool.Execute(context.Background(), "")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if out == "" {
		t.Fatal("Expected non-empty output from sys_info")
	}
}
