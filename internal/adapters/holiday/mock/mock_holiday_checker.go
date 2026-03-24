package mock

import (
	"sync"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

type key struct {
	date   string
	region entities.ClientRegion
}

type HolidayChecker struct {
	mu       sync.Mutex
	Holidays map[key]bool
	Error    error
}

func (m *HolidayChecker) SetHoliday(date time.Time, region entities.ClientRegion, isHoliday bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Holidays == nil {
		m.Holidays = make(map[key]bool)
	}

	m.Holidays[key{
		date:   normalizeDate(date).Format(time.DateOnly),
		region: region,
	}] = isHoliday
}

func (m *HolidayChecker) IsHoliday(date time.Time, region entities.ClientRegion) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Holidays == nil {
		return false, nil
	}

	return m.Holidays[key{
		date:   normalizeDate(date).Format(time.DateOnly),
		region: region,
	}], nil
}

func normalizeDate(date time.Time) time.Time {
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
