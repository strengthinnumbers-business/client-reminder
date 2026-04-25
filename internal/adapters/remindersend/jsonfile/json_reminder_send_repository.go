package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

type record struct {
	ClientID string                `json:"client_id"`
	Entry    entities.SendLogEntry `json:"entry"`
}

type state struct {
	Sends []record `json:"sends"`
}

type ReminderSendRepository struct {
	path string
	mu   sync.Mutex
}

func New(path string) *ReminderSendRepository {
	return &ReminderSendRepository{path: path}
}

func (r *ReminderSendRepository) ListSuccessfulSends(client entities.Client, period entities.Period) ([]entities.SendLogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return nil, err
	}

	var entries []entities.SendLogEntry
	for _, send := range state.Sends {
		if send.ClientID == client.ID && send.Entry.ForPeriod == period && send.Entry.Success {
			entries = append(entries, send.Entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].SequenceIndex == entries[j].SequenceIndex {
			return entries[i].SentAt.Before(entries[j].SentAt)
		}
		return entries[i].SequenceIndex < entries[j].SequenceIndex
	})

	return entries, nil
}

func (r *ReminderSendRepository) RecordSuccessfulSend(client entities.Client, entry entities.SendLogEntry) error {
	entry.Success = true
	return r.append(client, entry)
}

func (r *ReminderSendRepository) RecordFailedSend(client entities.Client, entry entities.SendLogEntry) error {
	entry.Success = false
	return r.append(client, entry)
}

func (r *ReminderSendRepository) append(client entities.Client, entry entities.SendLogEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return err
	}

	state.Sends = append(state.Sends, record{ClientID: client.ID, Entry: entry})
	return r.store(state)
}

func (r *ReminderSendRepository) load() (state, error) {
	bytes, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return state{}, nil
		}
		return state{}, fmt.Errorf("read reminder send state: %w", err)
	}

	if len(bytes) == 0 {
		return state{}, nil
	}

	var state state
	if err := json.Unmarshal(bytes, &state); err != nil {
		return state, fmt.Errorf("decode reminder send state: %w", err)
	}
	return state, nil
}

func (r *ReminderSendRepository) store(state state) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("create reminder send state directory: %w", err)
	}

	bytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode reminder send state: %w", err)
	}

	if err := os.WriteFile(r.path, bytes, 0o644); err != nil {
		return fmt.Errorf("write reminder send state: %w", err)
	}
	return nil
}
