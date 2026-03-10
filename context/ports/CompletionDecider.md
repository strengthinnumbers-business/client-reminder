# The `CompletionDecider` port

```go
type CompletionDecider interface {
	IsCompleted(c Customer, p Period) (CompletionVerdict, error)
	ResetCompletionVerdict(c Customer, p Period) error
}
```
