package entities_test

import (
	"testing"
	"time"

	holidaymock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/holiday/mock"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestMinimumBusinessDayGapsEffective(t *testing.T) {
	if got := (entities.MinimumBusinessDayGaps{}).Effective(); len(got) != len(entities.ReminderGapsStandard) {
		t.Fatalf("expected default reminder gaps length %d, got %d", len(entities.ReminderGapsStandard), len(got))
	}

	custom := entities.MinimumBusinessDayGaps{1, 4}
	got := custom.Effective()
	if len(got) != 2 || got[0] != 1 || got[1] != 4 {
		t.Fatalf("unexpected effective reminder gaps: %v", got)
	}
}

func TestPeriodFirstSequenceDay(t *testing.T) {
	holidays := &holidaymock.HolidayChecker{}

	t.Run("monthly", func(t *testing.T) {
		period := entities.CurrentPeriod(entities.PeriodMonthly, time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC))
		got, err := period.FirstSequenceDay(entities.RegionOntario, holidays)
		if err != nil {
			t.Fatalf("FirstSequenceDay returned error: %v", err)
		}
		want := time.Date(2026, time.February, 2, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("expected %s, got %s", want.Format(time.DateOnly), got.Format(time.DateOnly))
		}
	})

	t.Run("weekly", func(t *testing.T) {
		period := entities.CurrentPeriod(entities.PeriodWeekly, time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC))
		got, err := period.FirstSequenceDay(entities.RegionOntario, holidays)
		if err != nil {
			t.Fatalf("FirstSequenceDay returned error: %v", err)
		}
		want := time.Date(2026, time.February, 9, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("expected %s, got %s", want.Format(time.DateOnly), got.Format(time.DateOnly))
		}
	})

	t.Run("quarterly", func(t *testing.T) {
		period := entities.CurrentPeriod(entities.PeriodQuarterly, time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC))
		got, err := period.FirstSequenceDay(entities.RegionOntario, holidays)
		if err != nil {
			t.Fatalf("FirstSequenceDay returned error: %v", err)
		}
		want := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("expected %s, got %s", want.Format(time.DateOnly), got.Format(time.DateOnly))
		}
	})

	t.Run("holiday shifts first monday", func(t *testing.T) {
		holidays := &holidaymock.HolidayChecker{}
		holidays.SetHoliday(time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC), entities.RegionOntario, true)

		period := entities.CurrentPeriod(entities.PeriodMonthly, time.Date(2026, time.June, 3, 8, 0, 0, 0, time.UTC))
		got, err := period.FirstSequenceDay(entities.RegionOntario, holidays)
		if err != nil {
			t.Fatalf("FirstSequenceDay returned error: %v", err)
		}
		want := time.Date(2026, time.June, 2, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("expected %s, got %s", want.Format(time.DateOnly), got.Format(time.DateOnly))
		}
	})
}

func TestAddBusinessDays(t *testing.T) {
	holidays := &holidaymock.HolidayChecker{}
	holidays.SetHoliday(time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC), entities.RegionOntario, true)

	got, err := entities.AddBusinessDays(
		time.Date(2026, time.March, 27, 8, 0, 0, 0, time.UTC),
		2,
		entities.RegionOntario,
		holidays,
	)
	if err != nil {
		t.Fatalf("AddBusinessDays returned error: %v", err)
	}

	want := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want.Format(time.DateOnly), got.Format(time.DateOnly))
	}
}

func TestReminderScheduleNextEligibility(t *testing.T) {
	holidays := &holidaymock.HolidayChecker{}
	schedule := entities.ReminderSchedule{
		PeriodType:   entities.PeriodMonthly,
		Region:       entities.RegionOntario,
		ReminderGaps: entities.MinimumBusinessDayGaps{0, 3, 2, 2},
	}

	t.Run("first reminder can catch up after missed first sequence day", func(t *testing.T) {
		eligibility, ok, err := schedule.NextEligibility(
			time.Date(2026, time.February, 4, 8, 0, 0, 0, time.UTC),
			nil,
			holidays,
		)
		if err != nil {
			t.Fatalf("NextEligibility returned error: %v", err)
		}
		if !ok {
			t.Fatalf("expected reminder eligibility")
		}
		if eligibility.Period.ID != "2026-02" || eligibility.SequenceIndex != 0 {
			t.Fatalf("unexpected eligibility: %+v", eligibility)
		}
	})

	t.Run("next reminder waits for gap from actual previous send", func(t *testing.T) {
		previousSends := []entities.SendLogEntry{
			{
				ForPeriod:     entities.CurrentPeriod(entities.PeriodMonthly, time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC)),
				SequenceIndex: 0,
				SentAt:        time.Date(2026, time.February, 4, 8, 0, 0, 0, time.UTC),
				Success:       true,
			},
		}

		if _, ok, err := schedule.NextEligibility(time.Date(2026, time.February, 6, 8, 0, 0, 0, time.UTC), previousSends, holidays); err != nil || ok {
			t.Fatalf("expected no eligibility before gap passed, got ok=%v err=%v", ok, err)
		}

		eligibility, ok, err := schedule.NextEligibility(time.Date(2026, time.February, 9, 8, 0, 0, 0, time.UTC), previousSends, holidays)
		if err != nil {
			t.Fatalf("NextEligibility returned error: %v", err)
		}
		if !ok || eligibility.SequenceIndex != 1 {
			t.Fatalf("expected sequence index 1 eligibility, got ok=%v eligibility=%+v", ok, eligibility)
		}
	})

	t.Run("weekend before sequence start date returns false", func(t *testing.T) {
		weekly := entities.ReminderSchedule{
			PeriodType:   entities.PeriodWeekly,
			Region:       entities.RegionOntario,
			ReminderGaps: entities.MinimumBusinessDayGaps{0},
		}

		if _, ok, err := weekly.NextEligibility(time.Date(2026, time.February, 8, 8, 0, 0, 0, time.UTC), nil, holidays); err != nil || ok {
			t.Fatalf("expected no eligibility before sequence start, got ok=%v err=%v", ok, err)
		}
	})

	t.Run("exhausted gaps return false", func(t *testing.T) {
		schedule := entities.ReminderSchedule{
			PeriodType:   entities.PeriodMonthly,
			Region:       entities.RegionOntario,
			ReminderGaps: entities.MinimumBusinessDayGaps{0},
		}
		previousSends := []entities.SendLogEntry{
			{
				ForPeriod:     entities.CurrentPeriod(entities.PeriodMonthly, time.Date(2026, time.February, 2, 8, 0, 0, 0, time.UTC)),
				SequenceIndex: 0,
				SentAt:        time.Date(2026, time.February, 2, 8, 0, 0, 0, time.UTC),
				Success:       true,
			},
		}

		if _, ok, err := schedule.NextEligibility(time.Date(2026, time.February, 3, 8, 0, 0, 0, time.UTC), previousSends, holidays); err != nil || ok {
			t.Fatalf("expected no eligibility after exhausted gaps, got ok=%v err=%v", ok, err)
		}
	})
}
