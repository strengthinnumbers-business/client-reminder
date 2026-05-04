package ports

type Logger interface {
	Demo(message string, args ...any)
	Debug(message string, args ...any)
	Info(message string, args ...any)
	Error(message string, args ...any)
}

type NoopLogger struct{}

func (NoopLogger) Demo(message string, args ...any) {
}

func (NoopLogger) Debug(message string, args ...any) {
}

func (NoopLogger) Info(message string, args ...any) {
}

func (NoopLogger) Error(message string, args ...any) {
}

func EnsureLogger(logger Logger) Logger {
	if logger == nil {
		return NoopLogger{}
	}
	return logger
}
