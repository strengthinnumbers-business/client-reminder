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
	Verdicts map[key]entities.CompletionVerdictStatus
	Error    error
	Resets   []key
	Requests []Request
}

type Request struct {
	ClientID       string
	PeriodID       string
	ChangesSummary string
}

func (m *CompletionDecider) SetVerdict(customerID, periodID string, verdict entities.CompletionVerdictStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Verdicts == nil {
		m.Verdicts = make(map[key]entities.CompletionVerdictStatus)
	}
	m.Verdicts[key{customerID: customerID, periodID: periodID}] = verdict
}

func (m *CompletionDecider) GetVerdict(c entities.Client, p entities.Period) (entities.CompletionVerdictTask, error) {
	if m.Error != nil {
		return entities.CompletionVerdictTask{Status: entities.CompletionVerdictNotRequested}, m.Error
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	k := key{customerID: c.ID, periodID: p.ID}
	if m.Verdicts == nil {
		return entities.CompletionVerdictTask{ID: stateKey(k), Status: entities.CompletionVerdictNotRequested}, nil
	}
	v, ok := m.Verdicts[k]
	if !ok {
		return entities.CompletionVerdictTask{ID: stateKey(k), Status: entities.CompletionVerdictNotRequested}, nil
	}
	return entities.CompletionVerdictTask{ID: stateKey(k), Status: v}, nil
}

func (m *CompletionDecider) RequestNewCompletionVerdict(c entities.Client, p entities.Period, changesSummary string) (entities.CompletionVerdictTask, error) {
	if m.Error != nil {
		return entities.CompletionVerdictTask{}, m.Error
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	k := key{customerID: c.ID, periodID: p.ID}
	m.Resets = append(m.Resets, k)
	m.Requests = append(m.Requests, Request{ClientID: c.ID, PeriodID: p.ID, ChangesSummary: changesSummary})
	if m.Verdicts != nil {
		m.Verdicts[k] = entities.CompletionVerdictNotRequested
	}

	return entities.CompletionVerdictTask{ID: stateKey(k), Status: entities.CompletionVerdictNotRequested, ChangesSummary: changesSummary}, nil
}

func stateKey(k key) string {
	return k.customerID + "::" + k.periodID
}
