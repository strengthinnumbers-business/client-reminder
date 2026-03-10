package ports

import "github.com/strengthinnumbers-business/client-reminder/internal/core/entities"

// CompletionDecider decides whether a customer's current period is complete.
type CompletionDecider interface {
	IsCompleted(c entities.Client, p entities.Period) (entities.CompletionVerdict, error)
	ResetCompletionVerdict(c entities.Client, p entities.Period) error
}
