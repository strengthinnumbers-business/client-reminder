package ports

import "github.com/strengthinnumbers-business/client-reminder/internal/core/entities"

type UploadSnapshotter interface {
	GetPreviousAndCurrentSnapshot() (entities.UploadSnapshot, entities.UploadSnapshot, error)
}
