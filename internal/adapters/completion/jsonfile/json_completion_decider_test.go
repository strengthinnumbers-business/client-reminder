package jsonfile

import (
	"path/filepath"
	"testing"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestCompletionDecider_MissingVerdictDefaultsToNotRequested(t *testing.T) {
	decider := New(filepath.Join(t.TempDir(), "completion-verdicts.json"))

	verdict, err := decider.IsCompleted(
		entities.Client{ID: "c1"},
		entities.Period{Type: entities.PeriodMonthly, ID: "2026-02"},
	)
	if err != nil {
		t.Fatalf("IsCompleted returned error: %v", err)
	}
	if verdict != entities.CompletionVerdictNotRequested {
		t.Fatalf("expected CompletionVerdictNotRequested, got %v", verdict)
	}
}
