package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/fzihak/aethercore/core"
	"github.com/fzihak/aethercore/sdk/scaffold"
)

// handleScaffoldCmd parses 'aether scaffold' sub-flags and generates a new
// Layer 1 Module project from the SDK's embedded templates.
func handleScaffoldCmd(args []string) {
	scaffoldCmd := flag.NewFlagSet("scaffold", flag.ContinueOnError)
	name := scaffoldCmd.String("name", "", "Module name in kebab-case (e.g. web-search) [required]")
	author := scaffoldCmd.String("author", "", "Author name or organisation")
	version := scaffoldCmd.String("version", "0.1.0", "Starting semantic version for the module")
	output := scaffoldCmd.String("output", "", "Output directory (defaults to the module name)")
	scaffoldCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: aether scaffold --name <module-name> [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Generates a ready-to-build AetherCore Layer 1 Module scaffold.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		scaffoldCmd.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  aether scaffold --name web-search --author \"Jane Doe\"\n")
	}

	if err := scaffoldCmd.Parse(args); err != nil {
		core.Logger().Error("scaffold_parse_flags_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if *name == "" {
		fmt.Fprintln(os.Stderr, "Error: --name is required")
		scaffoldCmd.Usage()
		os.Exit(1)
	}

	outDir := *output
	if outDir == "" {
		outDir = *name
	}

	cfg := scaffold.Config{
		ModuleName: *name,
		Author:     *author,
		Version:    *version,
		OutputDir:  outDir,
	}

	core.Logger().Info("scaffolding_module",
		slog.String("name", *name),
		slog.String("output", outDir),
	)

	if err := scaffold.Generate(cfg); err != nil {
		core.Logger().Error("scaffold_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	fmt.Printf("Module %q scaffolded in %s\n", *name, outDir)
	fmt.Println()
	fmt.Printf("  cd %s\n", outDir)
	fmt.Printf("  # Edit go.mod: replace YOUR_USERNAME with your GitHub handle\n")
	fmt.Printf("  go mod tidy\n")
	fmt.Printf("  go build ./...\n")
}
