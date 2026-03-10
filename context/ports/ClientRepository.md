# The `ClientRepository` port

```go
type ClientRepository interface {
	GetAllClients() ([]Client, error)
}
```
