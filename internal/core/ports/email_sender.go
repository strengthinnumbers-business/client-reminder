package ports

type EmailSender interface {
	SendEmail(emails []string, subjectLine, textBody string) error
}
