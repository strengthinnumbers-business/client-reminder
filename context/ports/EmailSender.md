# The `EmailSender` port

```go
type EmailSender interface {
	SendEmail(email, textBody string) error
}
```
