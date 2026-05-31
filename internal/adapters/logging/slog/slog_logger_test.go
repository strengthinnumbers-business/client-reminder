package slog

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestNewTextFiltersLogsBelowMinimumLevel(t *testing.T) {
	var output bytes.Buffer
	logger := NewText(&output, LevelError)

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Error("error message")

	logs := output.String()
	if strings.Contains(logs, "debug message") {
		t.Fatalf("expected debug logs to be filtered, got %q", logs)
	}
	if strings.Contains(logs, "info message") {
		t.Fatalf("expected info logs to be filtered, got %q", logs)
	}
	if !strings.Contains(logs, "error message") {
		t.Fatalf("expected error logs to be written, got %q", logs)
	}
}

func TestNewTextAllowsDebugLevel(t *testing.T) {
	var output bytes.Buffer
	logger := NewText(&output, LevelDebug)

	logger.Debug("debug message")

	if !strings.Contains(output.String(), "debug message") {
		t.Fatalf("expected debug logs to be written, got %q", output.String())
	}
}

func TestDemoMethodsEmitSourceMapKeyOnly(t *testing.T) {
	var output bytes.Buffer
	logger := NewText(&output, LevelDemo)

	logger.DemoBelow("below message", "client_id", "abc")
	logger.DemoAbove("above message", "count", 2)
	logger.DemoSurrounding("surrounding message", "period", "2026-05")

	logs := output.String()
	for _, message := range []string{"below message", "above message", "surrounding message"} {
		if !strings.Contains(logs, message) {
			t.Fatalf("expected demo message %q in logs %q", message, logs)
		}
	}
	if !regexp.MustCompile(`source_map_key=[a-f0-9]{12}`).MatchString(logs) {
		t.Fatalf("expected short source_map_key in logs %q", logs)
	}
	for _, forbidden := range []string{"source_file", "source_line", "source_function", "source_message"} {
		if strings.Contains(logs, forbidden) {
			t.Fatalf("did not expect %s in logs %q", forbidden, logs)
		}
	}
}
