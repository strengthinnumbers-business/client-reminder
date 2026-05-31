package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type verdictMap map[string]entities.CompletionVerdictStatus

type CompletionDecider struct {
	path   string
	logger ports.Logger
	mu     sync.Mutex
}

type Option func(*CompletionDecider)

func New(path string, options ...Option) *CompletionDecider {
	d := &CompletionDecider{path: path, logger: ports.NoopLogger{}}
	for _, option := range options {
		option(d)
	}
	d.logger = ports.EnsureLogger(d.logger)
	return d
}

func WithLogger(logger ports.Logger) Option {
	return func(d *CompletionDecider) {
		d.logger = ports.EnsureLogger(logger)
	}
}

func (d *CompletionDecider) GetVerdict(c entities.Client, p entities.Period) (entities.CompletionVerdictTask, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, err := d.load()
	if err != nil {
		return entities.CompletionVerdictTask{Status: entities.CompletionUndecided}, err
	}

	id := stateKey(c.ID, p.ID)
	v, ok := state[stateKey(c.ID, p.ID)]
	if !ok {
		return entities.CompletionVerdictTask{ID: id, Status: entities.CompletionVerdictNotRequested}, nil
	}

	return entities.CompletionVerdictTask{ID: id, Status: v}, nil
}

func (d *CompletionDecider) RequestNewCompletionVerdict(c entities.Client, p entities.Period, changesSummary string) (entities.CompletionVerdictTask, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, err := d.load()
	if err != nil {
		return entities.CompletionVerdictTask{}, err
	}

	id := stateKey(c.ID, p.ID)
	state[id] = entities.CompletionVerdictNotRequested
	if err := d.store(state); err != nil {
		return entities.CompletionVerdictTask{}, err
	}

	return entities.CompletionVerdictTask{ID: id, Status: entities.CompletionVerdictNotRequested, ChangesSummary: changesSummary}, nil
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
