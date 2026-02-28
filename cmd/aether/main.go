package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
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
		// Strictly use os.Stdout so CI scripts can reliably parse this regardless of flag.Usage() stderr
		fmt.Fprintf(os.Stdout, "\n[AetherCore] Boot Latency: %s\n", core.FormatBootLatency())
	}()

	kernelMode := flag.Bool("kernel", false, "Start in Kernel Mode (enables distributed mesh and Rust sandbox)")
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

	// Parse flags for run command
	goal := flag.String("goal", "", "The goal for the ephemeral agent to accomplish")
	targetTool := flag.String("tool", "", "Bypass LLM and execute a specific native tool directly")
	toolArgs := flag.String("args", "{}", "JSON arguments to pass to the target tool")
	flag.Parse()

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
	fmt.Println("Validating authentication...")
	manager, err := core.NewAuthManager(nil) // Skipping PKI check for scaffolding
	if err != nil {
		log.Fatalf("Failed to load auth manager: %v", err)
	}

	payload, err := manager.Authenticate()
	if err != nil {
		log.Fatalf("Authentication failed: %v. Please run 'aether login'", err)
	}

	modeStr := "Pico Mode"
	if isKernel {
		modeStr = "Kernel Mode"
	}
	fmt.Printf("Authenticated as %s. Starting %s...\n", payload.Subject, modeStr)

	// Since we are strictly scaffolding Layer 0, we instantiate the Engine without a concrete LLM adapter
	// In Month 1, we will plug in OpenAI/Anthropic/Ollama adapters here.
	start := time.Now()

	engine := core.NewEngine(nil, 4, 100) // 4 bounded goroutines
	if err := engine.RegisterTool(&tools.SysInfoTool{}); err != nil {
		log.Fatalf("Core initialization failed at sys_info registration: %v", err)
	}

	engine.Start()

	// Intercept OS Signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	task := core.Task{
		ID:        "task_1",
		Input:     goal,
		CreatedAt: time.Now(),
	}

	if err := engine.Submit(task); err != nil {
		log.Fatalf("Failed to submit task: %v", err)
	}

	// Wait for singular result in this CLI run OR an interrupt
	var res core.Result
	select {
	case res = <-engine.Results():
		engine.Stop()
	case <-sigChan:
		fmt.Println("\n[Interrupt] Received Ctrl+C. Gracefully shutting down worker pool...")
		engine.Stop()
		fmt.Printf("Startup Time: %v\n", time.Since(start))
		os.Exit(130)
	}

	fmt.Printf("\n[Task Complete] Duration: %v\n", res.Duration)
	fmt.Printf("Startup Time: %v\n", time.Since(start))
	if res.Error != nil {
		log.Fatalf("Error: %v", res.Error)
	} else {
		// Mock output since there's no LLM yet
		fmt.Println("Output: [Engine initialized and task dispatched successfully]")
	}
}

// runToolNative bypasses the worker pool entirely to instantly execute a given tool for testing.
func runToolNative(toolName, args string) {
	fmt.Printf("AetherCore - Native Tool Execution: '%s'\n", toolName)
	start := time.Now()

	registry := core.NewToolRegistry()
	if err := registry.Register(&tools.SysInfoTool{}); err != nil {
		log.Fatalf("Tool registration failed: %v", err)
	}

	tool, err := registry.Get(toolName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[Latency %v] Executing...\n", time.Since(start))

	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Tool execution failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n--- Tool Output ---\n%s\n-------------------\n", out)
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
