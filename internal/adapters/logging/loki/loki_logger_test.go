package loki

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggerPushesStructuredEntryToLoki(t *testing.T) {
	var got pushRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/push" {
			t.Fatalf("expected Loki push path, got %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	logger := New(server.URL, LevelDemo)
	logger.Info("sent reminder", "client_id", "abc", "count", 2)

	if len(got.Streams) != 1 {
		t.Fatalf("expected one stream, got %#v", got.Streams)
	}
	if got.Streams[0].Stream["app"] != "client-reminder" {
		t.Fatalf("expected app label, got %#v", got.Streams[0].Stream)
	}
	if got.Streams[0].Stream["level"] != "info" {
		t.Fatalf("expected level label, got %#v", got.Streams[0].Stream)
	}

	var line map[string]any
	if err := json.Unmarshal([]byte(got.Streams[0].Values[0][1]), &line); err != nil {
		t.Fatalf("decode log line: %v", err)
	}
	if line["message"] != "sent reminder" || line["client_id"] != "abc" || line["count"].(float64) != 2 {
		t.Fatalf("unexpected log line %#v", line)
	}
}

func TestLoggerFiltersBelowMinimumLevel(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	logger := New(server.URL, LevelError)
	logger.Info("hidden")
	logger.Error("visible")

	if requests != 1 {
		t.Fatalf("expected one Loki request, got %d", requests)
	}
}
