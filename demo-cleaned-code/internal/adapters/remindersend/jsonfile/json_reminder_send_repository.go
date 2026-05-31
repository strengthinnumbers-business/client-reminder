package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type state struct {
	Sends []entities.SendLogEntry `json:"sends"`
}

type ReminderSendRepository struct {
	path   string
	logger ports.Logger
	mu     sync.Mutex
}

type Option func(*ReminderSendRepository)

func New(path string, options ...Option) *ReminderSendRepository {
	r := &ReminderSendRepository{path: path, logger: ports.NoopLogger{}}
	for _, option := range options {
		option(r)
	}
	r.logger = ports.EnsureLogger(r.logger)
	return r
}

func WithLogger(logger ports.Logger) Option {
	return func(r *ReminderSendRepository) {
		r.logger = ports.EnsureLogger(logger)
	}
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
		if send.ClientID == client.ID && send.ForPeriod == period && send.Success {
			entries = append(entries, send)
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
	entry.ClientID = client.ID
	entry.Success = true
	return r.append(entry)
}

func (r *ReminderSendRepository) RecordFailedSend(client entities.Client, entry entities.SendLogEntry) error {
	entry.ClientID = client.ID
	entry.Success = false
	return r.append(entry)
}

func (r *ReminderSendRepository) append(entry entities.SendLogEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return err
	}

	state.Sends = append(state.Sends, entry)
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
