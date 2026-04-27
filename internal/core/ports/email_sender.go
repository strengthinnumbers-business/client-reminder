package ports

type EmailSender interface {
	SendEmail(email, subjectLine, textBody string) error
}
