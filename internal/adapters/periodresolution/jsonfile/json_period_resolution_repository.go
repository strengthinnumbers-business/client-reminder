package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type record struct {
	ClientID    string          `json:"client_id"`
	Period      entities.Period `json:"period"`
	DealtWithAt time.Time       `json:"dealt_with_at"`
	Reason      string          `json:"reason"`
}

type state struct {
	Records []record `json:"records"`
}

type PeriodResolutionRepository struct {
	path   string
	logger ports.Logger
	mu     sync.Mutex
}

type Option func(*PeriodResolutionRepository)

func New(path string, options ...Option) *PeriodResolutionRepository {
	r := &PeriodResolutionRepository{path: path, logger: ports.NoopLogger{}}
	for _, option := range options {
		option(r)
	}
	r.logger = ports.EnsureLogger(r.logger)
	return r
}

func WithLogger(logger ports.Logger) Option {
	return func(r *PeriodResolutionRepository) {
		r.logger = ports.EnsureLogger(logger)
	}
}

func (r *PeriodResolutionRepository) IsDealtWith(client entities.Client, period entities.Period) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Demo("checking period resolution state", "path", r.path, "client_id", client.ID, "period", period.ID)
	state, err := r.load()
	if err != nil {
		return false, err
	}

	for _, record := range state.Records {
		if record.ClientID == client.ID && record.Period == period {
			r.logger.Demo("period resolution state says period is already dealt with", "path", r.path, "client_id", client.ID, "period", period.ID, "reason", record.Reason)
			return true, nil
		}
	}
	r.logger.Demo("period resolution state has no record for period", "path", r.path, "client_id", client.ID, "period", period.ID)
	return false, nil
}

func (r *PeriodResolutionRepository) MarkDealtWith(client entities.Client, period entities.Period, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Demo("marking period as dealt with", "path", r.path, "client_id", client.ID, "period", period.ID, "reason", reason)
	state, err := r.load()
	if err != nil {
		return err
	}

	for i := range state.Records {
		if state.Records[i].ClientID == client.ID && state.Records[i].Period == period {
			state.Records[i].DealtWithAt = time.Now().UTC()
			state.Records[i].Reason = reason
			r.logger.Demo("updated existing period resolution record", "path", r.path, "client_id", client.ID, "period", period.ID, "reason", reason)
			return r.store(state)
		}
	}

	state.Records = append(state.Records, record{
		ClientID:    client.ID,
		Period:      period,
		DealtWithAt: time.Now().UTC(),
		Reason:      reason,
	})
	r.logger.Demo("created period resolution record", "path", r.path, "client_id", client.ID, "period", period.ID, "reason", reason, "total_records", len(state.Records))
	return r.store(state)
}

func (r *PeriodResolutionRepository) load() (state, error) {
	bytes, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			r.logger.Demo("period resolution state file does not exist yet", "path", r.path)
			return state{}, nil
		}
		return state{}, fmt.Errorf("read period resolution state: %w", err)
	}

	if len(bytes) == 0 {
		r.logger.Demo("period resolution state file is empty", "path", r.path)
		return state{}, nil
	}

	var state state
	if err := json.Unmarshal(bytes, &state); err != nil {
		return state, fmt.Errorf("decode period resolution state: %w", err)
	}
	r.logger.Demo("loaded period resolution state file", "path", r.path, "records", len(state.Records))
	return state, nil
}

func (r *PeriodResolutionRepository) store(state state) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("create period resolution state directory: %w", err)
	}

	bytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode period resolution state: %w", err)
	}

	if err := os.WriteFile(r.path, bytes, 0o644); err != nil {
		return fmt.Errorf("write period resolution state: %w", err)
	}
	r.logger.Demo("wrote period resolution state file", "path", r.path, "records", len(state.Records))
	return nil
}
