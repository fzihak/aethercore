package tools

import (
	"context"
	"testing"

	"github.com/aethercore/aethercore/core"
)

func TestSysInfoTool(t *testing.T) {
	tool := &SysInfoTool{}

	manifest := tool.Manifest()
	if manifest.Name != "sys_info" {
		t.Fatalf("Expected Name sys_info, got %s", manifest.Name)
	}

	// Ensure capabilities are strictly constrained
	if len(manifest.Capabilities) != 1 || manifest.Capabilities[0] != core.CapState {
		t.Fatal("SysInfo tool should strictly only require CapState")
	}

	res, err := tool.Execute(context.Background(), "{}")
	if err != nil {
		t.Fatalf("Failed to execute sys_info: %v", err)
	}

	if len(res) < 10 {
		t.Fatalf("Response JSON unexpectedly small: %s", res)
	}
}
