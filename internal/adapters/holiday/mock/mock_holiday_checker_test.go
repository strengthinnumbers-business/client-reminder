package mock_test

import (
	"testing"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/adapters/holiday/mock"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestHolidayCheckerUsesCalendarDateOnly(t *testing.T) {
	checker := &mock.HolidayChecker{}
	checker.SetHoliday(
		time.Date(2026, time.July, 1, 8, 30, 0, 0, time.FixedZone("EDT", -4*60*60)),
		entities.RegionOntario,
		true,
	)

	isHoliday, err := checker.IsHoliday(
		time.Date(2026, time.July, 1, 22, 15, 0, 0, time.UTC),
		entities.RegionOntario,
	)
	if err != nil {
		t.Fatalf("IsHoliday returned error: %v", err)
	}
	if !isHoliday {
		t.Fatalf("expected holiday lookup to match on calendar date")
	}
}
