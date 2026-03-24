package ports

import (
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

type HolidayChecker interface {
	IsHoliday(date time.Time, region entities.ClientRegion) (bool, error)
}
