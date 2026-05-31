package email

import (
	"fmt"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type AdminAlerter struct {
	emailSender ports.EmailSender
	adminEmail  string
	logger      ports.Logger
}

type Option func(*AdminAlerter)

func New(emailSender ports.EmailSender, adminEmail string, options ...Option) *AdminAlerter {
	a := &AdminAlerter{
		emailSender: emailSender,
		adminEmail:  adminEmail,
		logger:      ports.NoopLogger{},
	}
	for _, option := range options {
		option(a)
	}
	a.logger = ports.EnsureLogger(a.logger)
	return a
}

func WithLogger(logger ports.Logger) Option {
	return func(a *AdminAlerter) {
		a.logger = ports.EnsureLogger(logger)
	}
}

func (a *AdminAlerter) AlertMissedPeriod(client entities.Client, period entities.Period, reason string) error {
	a.logger.DemoBelow("sending admin missed-period alert", "admin_email", a.adminEmail, "client_id", client.ID, "client_name", client.Name, "period", period.ID, "reason", reason)
	body := fmt.Sprintf(
		"Client reminder missed a whole period.\n\nClient: %s (%s)\nPeriod: %s\nReason: %s\n",
		client.Name,
		client.ID,
		period.ID,
		reason,
	)
	if err := a.emailSender.SendEmail(a.adminEmail, "Client reminder missed a whole period", body); err != nil {
		return err
	}
	a.logger.DemoAbove("sent admin missed-period alert", "admin_email", a.adminEmail, "client_id", client.ID, "client_name", client.Name, "period", period.ID)
	return nil
}
