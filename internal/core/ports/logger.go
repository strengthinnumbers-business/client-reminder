package ports

type Logger interface {
	Debug(message string, args ...any)
	Info(message string, args ...any)
	Error(message string, args ...any)
}

type NoopLogger struct{}

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
