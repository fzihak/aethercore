package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aethercore/aethercore/core"
	"github.com/aethercore/aethercore/core/tools"
)

const version = "0.1.0"

func main() {
	// 1. Initialize nanosecond precision telemetry before any allocations
	core.InitTelemetry()

	// 2. Guarantee telemetry is written to stdout on termination
	defer func() {
		core.Logger().Info("system_shutdown", slog.String("boot_latency", core.FormatBootLatency()))
	}()

	kernelMode := flag.Bool("kernel", false, "Start in Kernel Mode (enables distributed mesh and Rust sandbox)")
	logLevelStr := flag.String("log-level", "info", "Set the structured telemetry log level (debug, info, warn, error)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "AetherCore v%s - The Minimal Agent Kernel\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  aether onboard              First-time setup and authentication\n")
		fmt.Fprintf(os.Stderr, "  aether login                Re-authenticate if the token expired\n")
		fmt.Fprintf(os.Stderr, "  aether account delete       Delete your account from the Auth Cloud\n")
		fmt.Fprintf(os.Stderr, "  aether run --goal '...'     Execute a task using an ephemeral agent\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	// Parse global flags
	flag.Parse()

	// Initialize structured logger
	var level slog.Level
	switch strings.ToLower(*logLevelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	core.InitLogger(level)
	core.Logger().Debug("aethercore_boot_sequence_started")

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(0)
	}

	command := args[0]

	switch command {
	case "onboard":
		authCmd("signup")
	case "login":
		authCmd("login")
	case "account": // 'account delete'
		if len(args) > 1 && args[1] == "delete" {
			deleteCmd()
		} else {
			fmt.Println("Usage: aether account delete")
		}
	case "tool":
		if len(args) > 1 && args[1] == "list" {
			listToolsCmd()
		} else {
			fmt.Println("Usage: aether tool list")
		}
	case "run":
		runCmd := flag.NewFlagSet("run", flag.ContinueOnError)
		goal := runCmd.String("goal", "", "The goal for the ephemeral agent to accomplish")
		targetTool := runCmd.String("tool", "", "Bypass LLM and execute a specific native tool directly")
		toolArgs := runCmd.String("args", "{}", "JSON arguments to pass to the target tool")

		if err := runCmd.Parse(args[1:]); err != nil {
			core.Logger().Error("failed_to_parse_run_flags", slog.String("error", err.Error()))
			os.Exit(1)
		}

		if *targetTool != "" {
			runToolNative(*targetTool, *toolArgs)
			return
		}
		if *goal == "" {
			fmt.Println("Error: --goal is required for 'run' if not specifying a --tool")
			os.Exit(1)
		}
		runPicoMode(*goal, *kernelMode)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}

func runPicoMode(goal string, isKernel bool) {
	core.Logger().Debug("validating_authentication")
	manager, err := core.NewAuthManager(nil)
	if err != nil {
		core.Logger().Error("auth_manager_init_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	payload, err := manager.Authenticate()
	if err != nil {
		core.Logger().Error("authentication_failed", slog.String("error", err.Error()), slog.String("action", "run aether login"))
		os.Exit(1)
	}

	modeStr := "pico_mode"
	if isKernel {
		modeStr = "kernel_mode"
	}
	core.Logger().Info("engine_starting", slog.String("subject", payload.Subject), slog.String("mode", modeStr))

	// Since we are strictly scaffolding Layer 0, we instantiate the Engine without a concrete LLM adapter
	// In Month 1, we will plug in OpenAI/Anthropic/Ollama adapters here.
	start := time.Now()

	engine := core.NewEngine(nil, 4, 100)
	if err := engine.RegisterTool(&tools.SysInfoTool{}); err != nil {
		core.Logger().Error("tool_registration_failed", slog.String("tool", "sys_info"), slog.String("error", err.Error()))
		os.Exit(1)
	}

	engine.Start()

	// Intercept OS Signals for graceful shutdown (SIGINT & SIGTERM for K8s)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	task := core.Task{
		ID:        "task_1",
		Input:     goal,
		CreatedAt: time.Now(),
	}

	if err := engine.Submit(task); err != nil {
		core.Logger().Error("task_submission_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Wait for singular result in this CLI run OR an interrupt
	var res core.Result
	select {
	case res = <-engine.Results():
		engine.Stop()
	case <-sigChan:
		core.Logger().Warn("os_interrupt_received", slog.String("action", "shutting_down_worker_pool"))
		engine.Stop()
		core.Logger().Info("shutdown_complete", slog.Duration("uptime", time.Since(start)))
		os.Exit(130)
	}

	if res.Error != nil {
		core.Logger().Error("task_execution_failed", slog.String("error", res.Error.Error()), slog.Duration("duration", res.Duration))
		os.Exit(1)
	} else {
		core.Logger().Info("task_execution_success", slog.Duration("duration", res.Duration))
	}
}

// runToolNative bypasses the worker pool entirely to instantly execute a given tool for testing.
func runToolNative(toolName, args string) {
	core.Logger().Info("native_tool_execution_started", slog.String("tool", toolName))
	start := time.Now()

	registry := core.NewToolRegistry()
	if err := registry.Register(&tools.SysInfoTool{}); err != nil {
		core.Logger().Error("tool_registration_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	tool, err := registry.Get(toolName)
	if err != nil {
		core.Logger().Error("tool_not_found", slog.String("tool", toolName), slog.String("error", err.Error()))
		os.Exit(1)
	}

	core.Logger().Debug("executing_tool", slog.Duration("setup_latency", time.Since(start)))

	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		core.Logger().Error("tool_execution_failed", slog.String("tool", toolName), slog.String("error", err.Error()))
		os.Exit(1)
	}

	core.Logger().Info("tool_execution_success",
		slog.String("tool", toolName),
		slog.String("output", out),
		slog.Duration("duration", time.Since(start)),
	)
}

func listToolsCmd() {
	registry := core.NewToolRegistry()
	if err := registry.Register(&tools.SysInfoTool{}); err != nil {
		log.Fatalf("Tool registration failed: %v", err)
	}

	fmt.Println("Available Native Tools:")
	fmt.Println("---------------------------------------------------------")
	fmt.Printf("%-15s | %-12s | %-15s | %s\n", "NAME", "CAPABILITIES", "LIMITS (ms/MB)", "DESCRIPTION")
	fmt.Println("---------------------------------------------------------")

	manifests := registry.Manifests()
	for _, m := range manifests {
		caps := ""
		for _, c := range m.Capabilities {
			caps += string(c) + " "
		}

		limits := fmt.Sprintf("%dms / %dMB", m.MaxRuntimeMs, m.MemoryLimit)
		fmt.Printf("%-15s | %-12s | %-15s | %s\n", m.Name, caps, limits, m.Description)
	}
}
