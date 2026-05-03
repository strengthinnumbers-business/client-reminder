package slog

import (
	"bytes"
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
