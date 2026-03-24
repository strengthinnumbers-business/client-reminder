package service

import (
	"testing"
	"time"

	holidaymock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/holiday/mock"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestCalculateSequenceDayOffset(t *testing.T) {
	holidayChecker := &holidaymock.HolidayChecker{}
	svc := &ReminderService{holidayChecker: holidayChecker}

	t.Run("monthly business day offset", func(t *testing.T) {
		offset, ok, err := svc.calculateSequenceDayOffset(
			entities.PeriodMonthly,
			entities.RegionOntario,
			time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC),
		)
		if err != nil {
			t.Fatalf("calculateSequenceDayOffset returned error: %v", err)
		}
		if !ok {
			t.Fatalf("expected valid sequence day offset")
		}
		if offset != 6 {
			t.Fatalf("expected offset=6, got %d", offset)
		}
	})

	t.Run("weekend has no valid offset", func(t *testing.T) {
		offset, ok, err := svc.calculateSequenceDayOffset(
			entities.PeriodMonthly,
			entities.RegionOntario,
			time.Date(2026, time.February, 7, 8, 0, 0, 0, time.UTC),
		)
		if err != nil {
			t.Fatalf("calculateSequenceDayOffset returned error: %v", err)
		}
		if ok {
			t.Fatalf("expected no valid offset, got offset=%d", offset)
		}
	})

	t.Run("holiday shifts first sequence day", func(t *testing.T) {
		holidayChecker.SetHoliday(time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC), entities.RegionOntario, true)

		offset, ok, err := svc.calculateSequenceDayOffset(
			entities.PeriodMonthly,
			entities.RegionOntario,
			time.Date(2026, time.June, 3, 8, 0, 0, 0, time.UTC),
		)
		if err != nil {
			t.Fatalf("calculateSequenceDayOffset returned error: %v", err)
		}
		if !ok {
			t.Fatalf("expected valid sequence day offset")
		}
		if offset != 1 {
			t.Fatalf("expected offset=1, got %d", offset)
		}
	})
}
