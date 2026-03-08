package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- toPackageName / toGoTypeName helpers ------------------------------

func TestToPackageName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"web-search", "websearch"},
		{"my-module", "mymodule"},
		{"agent", "agent"},
		{"a-b-c", "abc"},
	}
	for _, tc := range cases {
		got := toPackageName(tc.in)
		if got != tc.want {
			t.Errorf("toPackageName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestToGoTypeName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"web-search", "WebSearch"},
		{"my-module", "MyModule"},
		{"agent", "Agent"},
		{"a-b-c", "ABC"},
	}
	for _, tc := range cases {
		got := toGoTypeName(tc.in)
		if got != tc.want {
			t.Errorf("toGoTypeName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ---- validateConfig ----------------------------------------------------

func TestValidateConfig_emptyName(t *testing.T) {
	if err := validateConfig(Config{}); err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}

func TestValidateConfig_nameWithSpace(t *testing.T) {
	if err := validateConfig(Config{ModuleName: "my module"}); err == nil {
		t.Fatal("expected error for name with space, got nil")
	}
}

func TestValidateConfig_validName(t *testing.T) {
	if err := validateConfig(Config{ModuleName: "web-search"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- Generate ----------------------------------------------------------

func TestGenerate_happyPath(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "my-module")

	cfg := Config{
		ModuleName: "my-module",
		Author:     "Test Author",
		Version:    "0.2.0",
		OutputDir:  outDir,
	}

	if err := Generate(cfg); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// The scaffold should produce at least 3 files
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) < 3 {
		t.Fatalf("expected at least 3 generated files, got %d", len(entries))
	}

	// Check the Go source file has the correct package name
	goFile := filepath.Join(outDir, "mymodule.go")
	content, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", goFile, err)
	}
	src := string(content)
	if !strings.Contains(src, "package mymodule") {
		t.Errorf("expected 'package mymodule' in generated Go file, got:\n%s", src)
	}
	if !strings.Contains(src, "type MyModule struct") {
		t.Errorf("expected 'type MyModule struct' in generated Go file")
	}
	if !strings.Contains(src, `"my-module"`) {
		t.Errorf("expected module name 'my-module' in manifest")
	}
	if !strings.Contains(src, "Test Author") {
		t.Errorf("expected author 'Test Author' in manifest")
	}
	if !strings.Contains(src, "0.2.0") {
		t.Errorf("expected version '0.2.0' in manifest")
	}

	// Check go.mod exists and has the module name
	goMod := filepath.Join(outDir, "go.mod")
	modContent, err := os.ReadFile(goMod)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", goMod, err)
	}
	if !strings.Contains(string(modContent), "my-module") {
		t.Errorf("expected 'my-module' in go.mod")
	}

	// Check README.md exists and mentions the module name
	readme := filepath.Join(outDir, "README.md")
	readmeContent, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", readme, err)
	}
	if !strings.Contains(string(readmeContent), "my-module") {
		t.Errorf("expected 'my-module' in README.md")
	}
}

func TestGenerate_emptyModuleName(t *testing.T) {
	if err := Generate(Config{OutputDir: t.TempDir()}); err == nil {
		t.Fatal("expected error for empty ModuleName, got nil")
	}
}

func TestGenerate_existingNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	// Pre-populate the output directory
	existingFile := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(existingFile, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := Config{ModuleName: "my-module", OutputDir: dir}
	if err := Generate(cfg); err == nil {
		t.Fatal("expected error for non-empty output directory, got nil")
	}
}

func TestGenerate_defaultOutputDir(t *testing.T) {
	// When OutputDir is empty, Generate should use the ModuleName as the directory.
	// We can't safely run this from the tests dir since it would write to cwd,
	// so we chdir to a temp dir first.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := Config{ModuleName: "hello-world", Author: "CI"}
	if err := Generate(cfg); err != nil {
		t.Fatalf("Generate with empty OutputDir: %v", err)
	}

	// Verify the directory was created with the module name
	if _, err := os.Stat(filepath.Join(tmp, "hello-world")); err != nil {
		t.Fatalf("expected 'hello-world' directory to be created: %v", err)
	}
}
