package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type ClientRepository struct {
	path   string
	logger ports.Logger
}

type Option func(*ClientRepository)

func New(path string, options ...Option) *ClientRepository {
	r := &ClientRepository{path: path, logger: ports.NoopLogger{}}
	for _, option := range options {
		option(r)
	}
	r.logger = ports.EnsureLogger(r.logger)
	return r
}

func WithLogger(logger ports.Logger) Option {
	return func(r *ClientRepository) {
		r.logger = ports.EnsureLogger(logger)
	}
}

func (r *ClientRepository) GetAllClients() ([]entities.Client, error) {
	bytes, err := os.ReadFile(r.path)
	if err != nil {
		return nil, fmt.Errorf("read clients json: %w", err)
	}

	var clients []entities.Client
	if err := json.Unmarshal(bytes, &clients); err != nil {
		return nil, fmt.Errorf("decode clients json: %w", err)
	}

	for i := range clients {
		if len(clients[i].ReminderGaps) == 0 {
			clients[i].ReminderGaps = entities.ReminderGapsStandard
		}
	}

	return clients, nil
}
