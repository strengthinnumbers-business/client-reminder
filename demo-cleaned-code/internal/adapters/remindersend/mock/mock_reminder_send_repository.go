package mock

import (
	"sort"
	"sync"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

type ReminderSendRepository struct {
	mu              sync.Mutex
	SuccessfulSends []RecordedSend
	FailedSends     []RecordedSend
	Error           error
}

type RecordedSend struct {
	ClientID string
	Entry    entities.SendLogEntry
}

func (r *ReminderSendRepository) ListSuccessfulSends(client entities.Client, period entities.Period) ([]entities.SendLogEntry, error) {
	if r.Error != nil {
		return nil, r.Error
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var entries []entities.SendLogEntry
	for _, send := range r.SuccessfulSends {
		if send.ClientID == client.ID && send.Entry.ForPeriod == period && send.Entry.Success {
			entries = append(entries, send.Entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].SequenceIndex == entries[j].SequenceIndex {
			return entries[i].SentAt.Before(entries[j].SentAt)
		}
		return entries[i].SequenceIndex < entries[j].SequenceIndex
	})
	return entries, nil
}

func (r *ReminderSendRepository) RecordSuccessfulSend(client entities.Client, entry entities.SendLogEntry) error {
	if r.Error != nil {
		return r.Error
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry.ClientID = client.ID
	entry.Success = true
	r.SuccessfulSends = append(r.SuccessfulSends, RecordedSend{ClientID: client.ID, Entry: entry})
	return nil
}

func (r *ReminderSendRepository) RecordFailedSend(client entities.Client, entry entities.SendLogEntry) error {
	if r.Error != nil {
		return r.Error
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry.ClientID = client.ID
	entry.Success = false
	r.FailedSends = append(r.FailedSends, RecordedSend{ClientID: client.ID, Entry: entry})
	return nil
}
