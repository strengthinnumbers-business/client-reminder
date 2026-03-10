package mock

import "sync"

type SentEmail struct {
	To   string
	Body string
}

type EmailSender struct {
	mu    sync.Mutex
	Sent  []SentEmail
	Error error
}

func (m *EmailSender) SendEmail(email, textBody string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Sent = append(m.Sent, SentEmail{To: email, Body: textBody})
	return m.Error
}
