package ports

// GlobalConfiguration provides app-wide static configuration values.
type GlobalConfiguration interface {
	GetEmailBodyTemplate() (string, error)
}
