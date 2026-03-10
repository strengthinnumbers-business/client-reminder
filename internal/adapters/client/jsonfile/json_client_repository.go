package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

type ClientRepository struct {
	path string
}

func New(path string) *ClientRepository {
	return &ClientRepository{path: path}
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
		if len(clients[i].Sequence) == 0 {
			clients[i].Sequence = entities.SequenceStandard
		}
	}

	return clients, nil
}
