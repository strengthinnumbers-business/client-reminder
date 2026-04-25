package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
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
	path string
	mu   sync.Mutex
}

func New(path string) *PeriodResolutionRepository {
	return &PeriodResolutionRepository{path: path}
}

func (r *PeriodResolutionRepository) IsDealtWith(client entities.Client, period entities.Period) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return false, err
	}

	for _, record := range state.Records {
		if record.ClientID == client.ID && record.Period == period {
			return true, nil
		}
	}
	return false, nil
}

func (r *PeriodResolutionRepository) MarkDealtWith(client entities.Client, period entities.Period, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return err
	}

	for i := range state.Records {
		if state.Records[i].ClientID == client.ID && state.Records[i].Period == period {
			state.Records[i].DealtWithAt = time.Now().UTC()
			state.Records[i].Reason = reason
			return r.store(state)
		}
	}

	state.Records = append(state.Records, record{
		ClientID:    client.ID,
		Period:      period,
		DealtWithAt: time.Now().UTC(),
		Reason:      reason,
	})
	return r.store(state)
}

func (r *PeriodResolutionRepository) load() (state, error) {
	bytes, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return state{}, nil
		}
		return state{}, fmt.Errorf("read period resolution state: %w", err)
	}

	if len(bytes) == 0 {
		return state{}, nil
	}

	var state state
	if err := json.Unmarshal(bytes, &state); err != nil {
		return state, fmt.Errorf("decode period resolution state: %w", err)
	}
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
	return nil
}
