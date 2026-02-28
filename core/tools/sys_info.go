package tools

import (
	"context"
	"encoding/json"
	"runtime"
	"time"

	"github.com/aethercore/aethercore/core"
)

// SysInfoTool provides baseline hardware and runtime heuristics for the agent.
type SysInfoTool struct{}

// Manifest strictly defines the capability bounds of this native tool.
func (s *SysInfoTool) Manifest() core.ToolManifest {
	return core.ToolManifest{
		Name:         "sys_info",
		Description:  "Retrieves critical operating system and hardware runtime telemetry",
		Parameters:   json.RawMessage(`{ "type": "object", "properties": {} }`),
		Capabilities: []core.Capability{core.CapState}, // Does not require network or disk IO
		MaxRuntimeMs: 100,
		MemoryLimit:  2,
	}
}

// Execute performs the logic of the tool in an ephemeral context.
func (s *SysInfoTool) Execute(ctx context.Context, args string) (string, error) {
	// For this native tool, we ignore args since parameters are empty.

	stats := struct {
		OS          string `json:"os"`
		Arch        string `json:"architecture"`
		CPUs        int    `json:"logical_cores"`
		Goroutines  int    `json:"active_goroutines"`
		CurrentTime string `json:"system_time_utc"`
	}{
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		CPUs:        runtime.NumCPU(),
		Goroutines:  runtime.NumGoroutine(),
		CurrentTime: time.Now().UTC().Format(time.RFC3339),
	}

	b, err := json.Marshal(stats)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
