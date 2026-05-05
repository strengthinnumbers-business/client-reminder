package ports

import "github.com/strengthinnumbers-business/client-reminder/internal/core/entities"

// CompletionDecider reports whether uploaded files for a period have an active or resolved verdict.
type CompletionDecider interface {
	GetVerdict(c entities.Client, p entities.Period) (entities.CompletionVerdictTask, error)
	RequestNewCompletionVerdict(c entities.Client, p entities.Period, changesSummary string) (entities.CompletionVerdictTask, error)
}
