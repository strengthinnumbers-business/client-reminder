package democlean

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRemovesDemoCallsAndWritesSourceMap(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "demo-cleaned-code")
	writeFile(t, sourceDir, "go.mod", "module example.com/demo\n\ngo 1.26.0\n")
	writeFile(t, sourceDir, "internal/example/example.go", `package example

type logger interface {
	DemoAbove(message string, args ...any)
	DemoBelow(message string, args ...any)
	DemoSurrounding(message string, args ...any)
}

func Run(logger logger) {
	value := 1
	logger.DemoAbove("loaded value", "value", value)
	if value > 0 {
		logger.DemoSurrounding("inside branch", "value", value)
		value++
	}
	logger.DemoBelow("printing value", "value", value)
	println(value)
}
`)

	if err := Generate(Options{SourceDir: sourceDir, OutputDir: outputDir}); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	cleanBytes, err := os.ReadFile(filepath.Join(outputDir, "internal/example/example.go"))
	if err != nil {
		t.Fatalf("read clean file: %v", err)
	}
	clean := string(cleanBytes)
	if strings.Contains(clean, "logger.DemoAbove") || strings.Contains(clean, "logger.DemoBelow") || strings.Contains(clean, "logger.DemoSurrounding") {
		t.Fatalf("expected demo calls to be removed:\n%s", clean)
	}
	if !strings.Contains(clean, "value++") || !strings.Contains(clean, "println(value)") {
		t.Fatalf("expected non-demo code to remain:\n%s", clean)
	}

	sourceMapBytes, err := os.ReadFile(filepath.Join(outputDir, "demo-source-map.json"))
	if err != nil {
		t.Fatalf("read source map: %v", err)
	}
	if strings.Contains(string(sourceMapBytes), "vscode_url") {
		t.Fatalf("source map must not contain vscode_url:\n%s", string(sourceMapBytes))
	}

	var sourceMap map[string]Entry
	if err := json.Unmarshal(sourceMapBytes, &sourceMap); err != nil {
		t.Fatalf("decode source map: %v", err)
	}
	if len(sourceMap) != 3 {
		t.Fatalf("expected 3 source map entries, got %#v", sourceMap)
	}
	assertEntry(t, sourceMap, "loaded value", "above", "demo-cleaned-code/internal/example/example.go")
	assertEntry(t, sourceMap, "inside branch", "surrounding", "demo-cleaned-code/internal/example/example.go")
	assertEntry(t, sourceMap, "printing value", "below", "demo-cleaned-code/internal/example/example.go")

	goMod, err := os.ReadFile(filepath.Join(outputDir, "go.mod"))
	if err != nil {
		t.Fatalf("read nested go.mod: %v", err)
	}
	if !strings.Contains(string(goMod), "module demo-cleaned-code") {
		t.Fatalf("expected generated module file, got %q", string(goMod))
	}

	writeFile(t, sourceDir, "internal/example/example_test.go", `package example

func TestRun() {}
`)
	if err := Generate(Options{SourceDir: sourceDir, OutputDir: outputDir}); err != nil {
		t.Fatalf("Generate returned error after adding test file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "internal/example/example_test.go")); !os.IsNotExist(err) {
		t.Fatalf("expected generated copy to skip test files, stat error was %v", err)
	}
}

func TestGenerateFailsWhenLegacyDemoCallRemains(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "demo-cleaned-code")
	writeFile(t, sourceDir, "go.mod", "module example.com/demo\n\ngo 1.26.0\n")
	writeFile(t, sourceDir, "example.go", `package example

func Run(logger interface{ Demo(string, ...any) }) {
	logger.Demo("legacy")
}
`)

	err := Generate(Options{SourceDir: sourceDir, OutputDir: outputDir})
	if err == nil || !strings.Contains(err.Error(), "legacy Demo call") {
		t.Fatalf("expected legacy Demo call error, got %v", err)
	}
}

func assertEntry(t *testing.T, sourceMap map[string]Entry, message, mode, cleanPath string) {
	t.Helper()
	for _, entry := range sourceMap {
		if entry.Message != message {
			continue
		}
		if entry.TargetMode != mode {
			t.Fatalf("expected mode %q for %q, got %#v", mode, message, entry)
		}
		if entry.CleanPath != cleanPath {
			t.Fatalf("expected clean path %q for %q, got %#v", cleanPath, message, entry)
		}
		if entry.CleanLine == 0 || entry.CleanColumn == 0 {
			t.Fatalf("expected clean position for %q, got %#v", message, entry)
		}
		return
	}
	t.Fatalf("missing source map entry for %q in %#v", message, sourceMap)
}

func writeFile(t *testing.T, root, relativePath, contents string) {
	t.Helper()
	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
