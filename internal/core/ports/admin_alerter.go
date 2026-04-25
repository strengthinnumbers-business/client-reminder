package ports

import "github.com/strengthinnumbers-business/client-reminder/internal/core/entities"

type AdminAlerter interface {
	AlertMissedPeriod(client entities.Client, period entities.Period, reason string) error
}
