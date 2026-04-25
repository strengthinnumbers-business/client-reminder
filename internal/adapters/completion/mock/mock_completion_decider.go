package mock

import (
	"sync"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

type key struct {
	customerID string
	periodID   string
}

type CompletionDecider struct {
	mu       sync.Mutex
	Verdicts map[key]entities.CompletionVerdict
	Error    error
	Resets   []key
}

func (m *CompletionDecider) SetVerdict(customerID, periodID string, verdict entities.CompletionVerdict) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Verdicts == nil {
		m.Verdicts = make(map[key]entities.CompletionVerdict)
	}
	m.Verdicts[key{customerID: customerID, periodID: periodID}] = verdict
}

func (m *CompletionDecider) IsCompleted(c entities.Client, p entities.Period) (entities.CompletionVerdict, error) {
	if m.Error != nil {
		return entities.CompletionVerdictNotRequested, m.Error
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Verdicts == nil {
		return entities.CompletionVerdictNotRequested, nil
	}
	v, ok := m.Verdicts[key{customerID: c.ID, periodID: p.ID}]
	if !ok {
		return entities.CompletionVerdictNotRequested, nil
	}
	return v, nil
}

func (m *CompletionDecider) ResetCompletionVerdict(c entities.Client, p entities.Period) error {
	if m.Error != nil {
		return m.Error
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	k := key{customerID: c.ID, periodID: p.ID}
	m.Resets = append(m.Resets, k)
	if m.Verdicts != nil {
		m.Verdicts[k] = entities.CompletionVerdictNotRequested
	}

	return nil
}
