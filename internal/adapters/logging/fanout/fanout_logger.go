package fanout

import "github.com/strengthinnumbers-business/client-reminder/internal/core/ports"

type Logger struct {
	loggers []ports.Logger
}

func New(loggers ...ports.Logger) *Logger {
	filtered := make([]ports.Logger, 0, len(loggers))
	for _, logger := range loggers {
		if logger != nil {
			filtered = append(filtered, logger)
		}
	}
	return &Logger{loggers: filtered}
}

func (l *Logger) Demo(message string, args ...any) {
	for _, logger := range l.loggers {
		logger.Demo(message, args...)
	}
}

func (l *Logger) Debug(message string, args ...any) {
	for _, logger := range l.loggers {
		logger.Debug(message, args...)
	}
}

func (l *Logger) Info(message string, args ...any) {
	for _, logger := range l.loggers {
		logger.Info(message, args...)
	}
}

func (l *Logger) Error(message string, args ...any) {
	for _, logger := range l.loggers {
		logger.Error(message, args...)
	}
}

var _ ports.Logger = (*Logger)(nil)
