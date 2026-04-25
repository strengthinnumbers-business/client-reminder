package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

type verdictMap map[string]entities.CompletionVerdict

type CompletionDecider struct {
	path string
	mu   sync.Mutex
}

func New(path string) *CompletionDecider {
	return &CompletionDecider{path: path}
}

func (d *CompletionDecider) IsCompleted(c entities.Client, p entities.Period) (entities.CompletionVerdict, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, err := d.load()
	if err != nil {
		return entities.CompletionUndecided, err
	}

	v, ok := state[stateKey(c.ID, p.ID)]
	if !ok {
		return entities.CompletionVerdictNotRequested, nil
	}

	return v, nil
}

func (d *CompletionDecider) ResetCompletionVerdict(c entities.Client, p entities.Period) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, err := d.load()
	if err != nil {
		return err
	}

	state[stateKey(c.ID, p.ID)] = entities.CompletionVerdictNotRequested
	if err := d.store(state); err != nil {
		return err
	}

	return nil
}

func (d *CompletionDecider) load() (verdictMap, error) {
	bytes, err := os.ReadFile(d.path)
	if err != nil {
		if os.IsNotExist(err) {
			return verdictMap{}, nil
		}
		return nil, fmt.Errorf("read completion state: %w", err)
	}

	if len(bytes) == 0 {
		return verdictMap{}, nil
	}

	var state verdictMap
	if err := json.Unmarshal(bytes, &state); err != nil {
		return nil, fmt.Errorf("decode completion state: %w", err)
	}
	if state == nil {
		state = verdictMap{}
	}
	return state, nil
}

func (d *CompletionDecider) store(state verdictMap) error {
	if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
		return fmt.Errorf("create completion state directory: %w", err)
	}

	bytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode completion state: %w", err)
	}

	if err := os.WriteFile(d.path, bytes, 0o644); err != nil {
		return fmt.Errorf("write completion state: %w", err)
	}

	return nil
}

func stateKey(customerID, periodID string) string {
	return customerID + "::" + periodID
}
