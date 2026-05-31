package smtp

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type EmailSender struct {
	host   string
	port   string
	from   string
	auth   smtp.Auth
	logger ports.Logger
}

type Option func(*EmailSender)

func New(host, port, username, password, from string, options ...Option) *EmailSender {
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	s := &EmailSender{host: host, port: port, from: from, auth: auth, logger: ports.NoopLogger{}}
	for _, option := range options {
		option(s)
	}
	s.logger = ports.EnsureLogger(s.logger)
	return s
}

func WithLogger(logger ports.Logger) Option {
	return func(s *EmailSender) {
		s.logger = ports.EnsureLogger(logger)
	}
}

func (s *EmailSender) SendEmail(email, subjectLine, textBody string) error {
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	s.logger.DemoBelow("sending SMTP email", "smtp_host", s.host, "smtp_port", s.port, "from", s.from, "to", email, "subject", subjectLine, "body_bytes", len(textBody))
	message := strings.Join([]string{
		fmt.Sprintf("From: %s", s.from),
		fmt.Sprintf("To: %s", email),
		fmt.Sprintf("Subject: %s", subjectLine),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=\"utf-8\"",
		"",
		textBody,
	}, "\r\n")

	if err := smtp.SendMail(addr, s.auth, s.from, []string{email}, []byte(message)); err != nil {
		return fmt.Errorf("send smtp mail: %w", err)
	}

	s.logger.DemoAbove("sent SMTP email", "smtp_host", s.host, "smtp_port", s.port, "from", s.from, "to", email, "subject", subjectLine)
	return nil
}
