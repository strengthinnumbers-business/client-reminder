package smtp

import (
	"fmt"
	"net/smtp"
	"strings"
)

type EmailSender struct {
	host string
	port string
	from string
	auth smtp.Auth
}

func New(host, port, username, password, from string) *EmailSender {
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	return &EmailSender{host: host, port: port, from: from, auth: auth}
}

func (s *EmailSender) SendEmail(email, subjectLine, textBody string) error {
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
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

	return nil
}
