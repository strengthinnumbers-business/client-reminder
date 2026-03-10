package mock

import "github.com/strengthinnumbers-business/client-reminder/internal/core/entities"

type ClientRepository struct {
	Clients []entities.Client
	Error   error
}

func (m *ClientRepository) GetAllClients() ([]entities.Client, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Clients, nil
}
