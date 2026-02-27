package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aethercore/aethercore/core"
)

const version = "0.1.0"

func main() {
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
	case "run":
		if *goal == "" {
			fmt.Println("Error: --goal is required for 'run'")
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
	engine.Start()

	task := core.Task{
		ID:        "task_1",
		Input:     goal,
		CreatedAt: time.Now(),
	}

	if err := engine.Submit(task); err != nil {
		log.Fatalf("Failed to submit task: %v", err)
	}

	// Wait for singular result in this CLI run
	res := <-engine.Results()
	engine.Stop()

	fmt.Printf("\n[Task Complete] Duration: %v\n", res.Duration)
	fmt.Printf("Startup Time: %v\n", time.Since(start))
	if res.Error != nil {
		log.Fatalf("Error: %v", res.Error)
	} else {
		// Mock output since there's no LLM yet
		fmt.Println("Output: [Engine initialized and task dispatched successfully]")
	}
}
