package jsonfile

import (
	"path/filepath"
	"testing"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestCompletionDecider_MissingVerdictDefaultsToNotRequested(t *testing.T) {
	decider := New(filepath.Join(t.TempDir(), "completion-verdicts.json"))

	task, err := decider.GetVerdict(
		entities.Client{ID: "c1"},
		entities.Period{Type: entities.PeriodMonthly, ID: "2026-02"},
	)
	if err != nil {
		t.Fatalf("GetVerdict returned error: %v", err)
	}
	if task.Status != entities.CompletionVerdictNotRequested {
		t.Fatalf("expected CompletionVerdictNotRequested, got %v", task.Status)
	}
}
