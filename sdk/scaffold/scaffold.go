// Package scaffold generates a ready-to-build AetherCore Layer 1 Module project
// from embedded Go templates.
//
// Usage:
//
//	cfg := scaffold.Config{
//	    ModuleName: "web-search",
//	    Author:     "Jane Doe",
//	    Version:    "0.1.0",
//	    OutputDir:  "./web-search",
//	}
//	if err := scaffold.Generate(cfg); err != nil {
//	    log.Fatal(err)
//	}
package scaffold

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

//go:embed templates/*
var templateFS embed.FS

// Config holds the parameters used when generating a module scaffold.
type Config struct {
	// ModuleName is the kebab-case module identifier (e.g. "web-search").
	// It is used as the directory name, Go module path suffix, and manifest name.
	ModuleName string

	// Author is the module developer's name or organisation.
	Author string

	// Version is the starting semantic version for the generated module (e.g. "0.1.0").
	Version string

	// OutputDir is the absolute or relative path where the scaffold will be written.
	// The directory is created if it does not exist.
	OutputDir string
}

// templateData carries the values injected into every template file.
type templateData struct {
	ModuleName  string // kebab-case identifier ("web-search")
	PackageName string // Go package name  ("websearch")
	GoTypeName  string // Exported Go type  ("WebSearch")
	Author      string
	Version     string
}

// Generate creates the scaffold directory and writes all template files.
// Returns an error if OutputDir already exists and is non-empty, or if any
// template execution fails.
func Generate(cfg Config) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}

	dir := cfg.OutputDir
	if dir == "" {
		dir = cfg.ModuleName
	}

	if err := checkOutputDir(dir); err != nil {
		return err
	}

	data := templateData{
		ModuleName:  cfg.ModuleName,
		PackageName: toPackageName(cfg.ModuleName),
		GoTypeName:  toGoTypeName(cfg.ModuleName),
		Author:      cfg.Author,
		Version:     cfg.Version,
	}

	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return fmt.Errorf("scaffold: failed to read embedded templates: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// embed.FS always uses forward slashes regardless of OS.
		src := path.Join("templates", entry.Name())
		dst := outputFileName(dir, entry.Name(), data.PackageName)

		if err := renderTemplate(src, dst, &data); err != nil {
			return err
		}
	}

	return nil
}

// validateConfig returns an error for obviously invalid configurations.
func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.ModuleName) == "" {
		return errors.New("scaffold: ModuleName must not be empty")
	}
	if strings.ContainsAny(cfg.ModuleName, " \t\n/\\") {
		return fmt.Errorf("scaffold: ModuleName %q must be a kebab-case identifier", cfg.ModuleName)
	}
	return nil
}

// checkOutputDir ensures the output directory is empty or does not yet exist.
func checkOutputDir(dir string) error {
	info, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(dir, 0o750) //nolint:gosec // G301: 0750 grants owner+group; world has no access
	}
	if err != nil {
		return fmt.Errorf("scaffold: stat %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("scaffold: output path %q exists and is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("scaffold: readdir %q: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("scaffold: output directory %q already exists and is non-empty", dir)
	}
	return nil
}

// outputFileName maps a template filename to its destination path.
// Templates are named like "module.go.tmpl" → "<dir>/<pkg>.go",
// "go.mod.tmpl" → "<dir>/go.mod", "README.md.tmpl" → "<dir>/README.md".
func outputFileName(dir, tmplName, pkgName string) string {
	name := strings.TrimSuffix(tmplName, ".tmpl")
	if name == "module.go" {
		name = pkgName + ".go"
	}
	return filepath.Join(dir, name)
}

// renderTemplate parses one embedded template and writes the result to dst.
func renderTemplate(src, dst string, data *templateData) error {
	raw, err := templateFS.ReadFile(src)
	if err != nil {
		return fmt.Errorf("scaffold: read template %q: %w", src, err)
	}

	tmpl, err := template.New(filepath.Base(src)).Parse(string(raw))
	if err != nil {
		return fmt.Errorf("scaffold: parse template %q: %w", src, err)
	}

	f, err := os.Create(dst) //nolint:gosec // G304: dst is filepath.Join(validatedOutputDir, knownTemplateName)
	if err != nil {
		return fmt.Errorf("scaffold: create %q: %w", dst, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("scaffold: execute template %q: %w", src, err)
	}
	return nil
}

// toPackageName converts a kebab-case name to a valid Go package name.
// "web-search" → "websearch".
func toPackageName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "-", ""))
}

// toGoTypeName converts a kebab-case name to an exported Go type name.
// "web-search" → "WebSearch".
func toGoTypeName(name string) string {
	var sb strings.Builder
	capitaliseNext := true
	for _, r := range name {
		if r == '-' {
			capitaliseNext = true
			continue
		}
		if capitaliseNext {
			sb.WriteRune(unicode.ToUpper(r))
			capitaliseNext = false
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
