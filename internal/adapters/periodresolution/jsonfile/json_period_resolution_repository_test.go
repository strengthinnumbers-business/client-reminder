package jsonfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestPeriodResolutionRepository_MissingFileStartsEmptyAndStoresResolution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "period-resolutions.json")
	repo := New(path)
	client := entities.Client{ID: "c1"}
	period := entities.Period{Type: entities.PeriodMonthly, ID: "2026-01"}

	dealtWith, err := repo.IsDealtWith(client, period)
	if err != nil {
		t.Fatalf("IsDealtWith returned error: %v", err)
	}
	if dealtWith {
		t.Fatalf("expected missing file period to be unresolved")
	}

	if err := repo.MarkDealtWith(client, period, "onboarding baseline: client added after this period"); err != nil {
		t.Fatalf("MarkDealtWith returned error: %v", err)
	}

	dealtWith, err = repo.IsDealtWith(client, period)
	if err != nil {
		t.Fatalf("IsDealtWith returned error: %v", err)
	}
	if !dealtWith {
		t.Fatalf("expected period to be dealt with")
	}
}

func TestPeriodResolutionRepository_CorruptFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "period-resolutions.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write corrupt state: %v", err)
	}

	_, err := New(path).IsDealtWith(entities.Client{ID: "c1"}, entities.Period{Type: entities.PeriodMonthly, ID: "2026-01"})
	if err == nil {
		t.Fatalf("expected corrupt state error")
	}
}
