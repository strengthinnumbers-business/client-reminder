package mock

import (
	"sync"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

type Alert struct {
	ClientID string
	Period   entities.Period
	Reason   string
}

type AdminAlerter struct {
	mu     sync.Mutex
	Alerts []Alert
	Error  error
}

func (a *AdminAlerter) AlertMissedPeriod(client entities.Client, period entities.Period, reason string) error {
	if a.Error != nil {
		return a.Error
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.Alerts = append(a.Alerts, Alert{ClientID: client.ID, Period: period, Reason: reason})
	return nil
}
