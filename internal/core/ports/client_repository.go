package ports

import "github.com/strengthinnumbers-business/client-reminder/internal/core/entities"

type ClientRepository interface {
	GetAllClients() ([]entities.Client, error)
}
