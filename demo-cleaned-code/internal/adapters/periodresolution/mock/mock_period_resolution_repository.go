package mock

import (
	"sync"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

type key struct {
	clientID string
	periodID string
}

type PeriodResolutionRepository struct {
	mu      sync.Mutex
	Records map[key]string
	Error   error
}

func (r *PeriodResolutionRepository) IsDealtWith(client entities.Client, period entities.Period) (bool, error) {
	if r.Error != nil {
		return false, r.Error
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.Records[key{clientID: client.ID, periodID: period.ID}]
	return ok, nil
}

func (r *PeriodResolutionRepository) MarkDealtWith(client entities.Client, period entities.Period, reason string) error {
	if r.Error != nil {
		return r.Error
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Records == nil {
		r.Records = make(map[key]string)
	}
	r.Records[key{clientID: client.ID, periodID: period.ID}] = reason
	return nil
}
