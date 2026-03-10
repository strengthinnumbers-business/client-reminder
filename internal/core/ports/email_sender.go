package ports

type EmailSender interface {
	SendEmail(email, textBody string) error
}
