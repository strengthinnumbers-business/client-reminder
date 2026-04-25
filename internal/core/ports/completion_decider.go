package ports

import "github.com/strengthinnumbers-business/client-reminder/internal/core/entities"

// CompletionDecider reports whether uploaded files for a period have an active or resolved verdict.
type CompletionDecider interface {
	IsCompleted(c entities.Client, p entities.Period) (entities.CompletionVerdict, error)
	ResetCompletionVerdict(c entities.Client, p entities.Period) error
}
