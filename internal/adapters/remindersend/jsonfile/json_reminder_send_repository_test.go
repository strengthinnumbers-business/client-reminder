package jsonfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestReminderSendRepository_MissingFileStartsEmptyAndStoresSends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reminder-sends.json")
	repo := New(path)
	client := entities.Client{ID: "c1"}
	period := entities.Period{Type: entities.PeriodMonthly, ID: "2026-02"}

	sends, err := repo.ListSuccessfulSends(client, period)
	if err != nil {
		t.Fatalf("ListSuccessfulSends returned error: %v", err)
	}
	if len(sends) != 0 {
		t.Fatalf("expected no sends for missing file, got %+v", sends)
	}

	if err := repo.RecordSuccessfulSend(client, entities.SendLogEntry{
		ForPeriod:     period,
		SequenceIndex: 0,
		SentAt:        time.Date(2026, time.February, 2, 8, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordSuccessfulSend returned error: %v", err)
	}
	if err := repo.RecordFailedSend(client, entities.SendLogEntry{
		ForPeriod:     period,
		SequenceIndex: 1,
		SentAt:        time.Date(2026, time.February, 5, 8, 0, 0, 0, time.UTC),
		ErrorMessage:  "smtp down",
	}); err != nil {
		t.Fatalf("RecordFailedSend returned error: %v", err)
	}

	sends, err = repo.ListSuccessfulSends(client, period)
	if err != nil {
		t.Fatalf("ListSuccessfulSends returned error: %v", err)
	}
	if len(sends) != 1 || sends[0].SequenceIndex != 0 {
		t.Fatalf("expected only successful send to be listed, got %+v", sends)
	}
	if sends[0].ClientID != client.ID {
		t.Fatalf("expected send to include client ID %q, got %+v", client.ID, sends[0])
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stored state: %v", err)
	}
	stored := string(bytes)
	if strings.Contains(stored, `"entry"`) || !strings.Contains(stored, `"ClientID": "c1"`) {
		t.Fatalf("expected flat SendLogEntry records in stored state, got %s", stored)
	}
}

func TestReminderSendRepository_CorruptFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reminder-sends.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write corrupt state: %v", err)
	}

	_, err := New(path).ListSuccessfulSends(entities.Client{ID: "c1"}, entities.Period{Type: entities.PeriodMonthly, ID: "2026-02"})
	if err == nil {
		t.Fatalf("expected corrupt state error")
	}
}
