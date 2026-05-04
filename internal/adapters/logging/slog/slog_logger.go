package slog

import (
	"io"
	"log/slog"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type Level string

const (
	LevelDemo  Level = "demo"
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelError Level = "error"
)

type Logger struct {
	logger *slog.Logger
}

func NewText(writer io.Writer, minimumLevel Level) *Logger {
	return New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: slogLevel(minimumLevel),
	}))
}

func NewJSON(writer io.Writer, minimumLevel Level) *Logger {
	return New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: slogLevel(minimumLevel),
	}))
}

func New(handler slog.Handler) *Logger {
	return &Logger{logger: slog.New(handler)}
}

func (l *Logger) Demo(message string, args ...any) {
	l.logger.Debug(message, args...)
}

func (l *Logger) Debug(message string, args ...any) {
	l.logger.Debug(message, args...)
}

func (l *Logger) Info(message string, args ...any) {
	l.logger.Info(message, args...)
}

func (l *Logger) Error(message string, args ...any) {
	l.logger.Error(message, args...)
}

func slogLevel(level Level) slog.Level {
	switch level {
	case LevelDemo:
		return slog.LevelDebug
	case LevelDebug:
		return slog.LevelDebug
	case LevelError:
		return slog.LevelError
	case LevelInfo, "":
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

var _ ports.Logger = (*Logger)(nil)
