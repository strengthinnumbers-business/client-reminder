package email

import (
	"fmt"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type AdminAlerter struct {
	emailSender ports.EmailSender
	adminEmail  string
}

func New(emailSender ports.EmailSender, adminEmail string) *AdminAlerter {
	return &AdminAlerter{
		emailSender: emailSender,
		adminEmail:  adminEmail,
	}
}

func (a *AdminAlerter) AlertMissedPeriod(client entities.Client, period entities.Period, reason string) error {
	body := fmt.Sprintf(
		"Client reminder missed a whole period.\n\nClient: %s (%s)\nPeriod: %s\nReason: %s\n",
		client.Name,
		client.ID,
		period.ID,
		reason,
	)
	return a.emailSender.SendEmail(a.adminEmail, body)
}
