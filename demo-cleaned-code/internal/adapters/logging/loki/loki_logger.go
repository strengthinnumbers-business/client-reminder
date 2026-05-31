package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
	"github.com/strengthinnumbers-business/client-reminder/internal/demolog"
)

const defaultTimeout = 2 * time.Second

type Level string

const (
	LevelDemo  Level = "demo"
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelError Level = "error"
)

type Logger struct {
	pushURL      string
	client       *http.Client
	minimumLevel Level
	labels       map[string]string
}

type Option func(*Logger)

func WithHTTPClient(client *http.Client) Option {
	return func(l *Logger) {
		if client != nil {
			l.client = client
		}
	}
}

func WithLabels(labels map[string]string) Option {
	return func(l *Logger) {
		for key, value := range labels {
			if key != "" && value != "" {
				l.labels[key] = value
			}
		}
	}
}

func New(endpoint string, minimumLevel Level, options ...Option) *Logger {
	logger := &Logger{
		pushURL:      strings.TrimRight(endpoint, "/") + "/loki/api/v1/push",
		client:       &http.Client{Timeout: defaultTimeout},
		minimumLevel: normalizeLevel(minimumLevel),
		labels: map[string]string{
			"app": "client-reminder",
		},
	}
	for _, option := range options {
		option(logger)
	}
	return logger
}

func (l *Logger) DemoBelow(message string, args ...any) {
	l.demo(message, args...)
}

func (l *Logger) DemoAbove(message string, args ...any) {
	l.demo(message, args...)
}

func (l *Logger) DemoSurrounding(message string, args ...any) {
	l.demo(message, args...)
}

func (l *Logger) demo(message string, args ...any) {
	args = append([]any{"source_map_key", demolog.CallerSourceMapKey(message)}, args...)
	l.write(LevelDemo, message, args...)
}

func (l *Logger) Debug(message string, args ...any) {
	l.write(LevelDebug, message, args...)
}

func (l *Logger) Info(message string, args ...any) {
	l.write(LevelInfo, message, args...)
}

func (l *Logger) Error(message string, args ...any) {
	l.write(LevelError, message, args...)
}

func (l *Logger) write(level Level, message string, args ...any) {
	level = normalizeLevel(level)
	if severity(level) < severity(l.minimumLevel) || l.pushURL == "/loki/api/v1/push" {
		return
	}

	entry := map[string]any{
		"time":    time.Now().UTC().Format(time.RFC3339Nano),
		"level":   string(level),
		"message": message,
	}
	for key, value := range fieldsFromArgs(args) {
		entry[key] = value
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return
	}

	labels := make(map[string]string, len(l.labels)+1)
	for key, value := range l.labels {
		labels[key] = value
	}
	labels["level"] = string(level)

	payload := pushRequest{
		Streams: []stream{
			{
				Stream: labels,
				Values: [][2]string{{
					strconv.FormatInt(time.Now().UnixNano(), 10),
					string(line),
				}},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.pushURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

func fieldsFromArgs(args []any) map[string]any {
	fields := make(map[string]any)
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok || key == "" {
			continue
		}
		fields[key] = jsonSafeValue(args[i+1])
	}
	return fields
}

func jsonSafeValue(value any) any {
	if value == nil {
		return nil
	}
	if err, ok := value.(error); ok {
		return err.Error()
	}
	if _, err := json.Marshal(value); err == nil {
		return value
	}
	return fmt.Sprint(value)
}

func normalizeLevel(level Level) Level {
	switch strings.ToLower(string(level)) {
	case string(LevelDemo):
		return LevelDemo
	case string(LevelDebug):
		return LevelDebug
	case string(LevelError):
		return LevelError
	case string(LevelInfo), "":
		return LevelInfo
	default:
		return LevelInfo
	}
}

func severity(level Level) int {
	switch level {
	case LevelDemo, LevelDebug:
		return 0
	case LevelError:
		return 2
	case LevelInfo:
		return 1
	default:
		return 1
	}
}

type pushRequest struct {
	Streams []stream `json:"streams"`
}

type stream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

var _ ports.Logger = (*Logger)(nil)
