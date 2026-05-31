package ports

import "github.com/strengthinnumbers-business/client-reminder/internal/core/entities"

type PeriodResolutionRepository interface {
	IsDealtWith(client entities.Client, period entities.Period) (bool, error)
	MarkDealtWith(client entities.Client, period entities.Period, reason string) error
}
