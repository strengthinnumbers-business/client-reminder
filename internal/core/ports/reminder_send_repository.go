package ports

import "github.com/strengthinnumbers-business/client-reminder/internal/core/entities"

type ReminderSendRepository interface {
	ListSuccessfulSends(client entities.Client, period entities.Period) ([]entities.SendLogEntry, error)
	RecordSuccessfulSend(client entities.Client, entry entities.SendLogEntry) error
	RecordFailedSend(client entities.Client, entry entities.SendLogEntry) error
}
