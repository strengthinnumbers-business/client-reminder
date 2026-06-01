package smtp

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type EmailSender struct {
	host     string
	port     string
	from     string
	auth     smtp.Auth
	sendMail func(string, smtp.Auth, string, []string, []byte) error
	logger   ports.Logger
}

type Option func(*EmailSender)

func New(host, port, username, password, from string, options ...Option) *EmailSender {
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	s := &EmailSender{host: host, port: port, from: from, auth: auth, sendMail: smtp.SendMail, logger: ports.NoopLogger{}}
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

func WithSendMail(sendMail func(string, smtp.Auth, string, []string, []byte) error) Option {
	return func(s *EmailSender) {
		if sendMail != nil {
			s.sendMail = sendMail
		}
	}
}

func (s *EmailSender) SendEmail(emails []string, subjectLine, textBody string) error {
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	message := strings.Join([]string{
		fmt.Sprintf("From: %s", s.from),
		fmt.Sprintf("To: %s", strings.Join(emails, ", ")),
		fmt.Sprintf("Subject: %s", subjectLine),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=\"utf-8\"",
		"",
		textBody,
	}, "\r\n")

	if err := s.sendMail(addr, s.auth, s.from, emails, []byte(message)); err != nil {
		return fmt.Errorf("send smtp mail: %w", err)
	}

	return nil
}
