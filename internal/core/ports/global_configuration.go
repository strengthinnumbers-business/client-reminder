package ports

// GlobalConfiguration provides app-wide static configuration values.
type GlobalConfiguration interface {
	GetEmailBodyTemplate(sequenceIndex int, style string) (string, string, error)
}
