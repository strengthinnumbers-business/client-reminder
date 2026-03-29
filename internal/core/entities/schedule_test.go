package entities_test

import (
	"testing"
	"time"

	holidaymock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/holiday/mock"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestSequenceDayOffsetsEffective(t *testing.T) {
	if got := (entities.SequenceDayOffsets{}).Effective(); len(got) != len(entities.SequenceStandard) {
		t.Fatalf("expected default sequence length %d, got %d", len(entities.SequenceStandard), len(got))
	}

	custom := entities.SequenceDayOffsets{1, 4}
	got := custom.Effective()
	if len(got) != 2 || got[0] != 1 || got[1] != 4 {
		t.Fatalf("unexpected effective sequence: %v", got)
	}
}

func TestSequenceDayOffsetsIndexOf(t *testing.T) {
	idx, ok := (entities.SequenceDayOffsets{0, 3, 5}).IndexOf(3)
	if !ok || idx != 1 {
		t.Fatalf("expected offset 3 at index 1, got ok=%v idx=%d", ok, idx)
	}

	if _, ok := (entities.SequenceDayOffsets{0, 3, 5}).IndexOf(7); ok {
		t.Fatalf("expected offset 7 to be missing")
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

func TestPeriodSequenceDayOffsetAt(t *testing.T) {
	holidays := &holidaymock.HolidayChecker{}
	period := entities.CurrentPeriod(entities.PeriodMonthly, time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC))

	t.Run("valid business day", func(t *testing.T) {
		offset, ok, err := period.SequenceDayOffsetAt(
			time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC),
			entities.RegionOntario,
			holidays,
		)
		if err != nil {
			t.Fatalf("SequenceDayOffsetAt returned error: %v", err)
		}
		if !ok || offset != 6 {
			t.Fatalf("expected ok=true offset=6, got ok=%v offset=%d", ok, offset)
		}
	})

	t.Run("weekend is invalid", func(t *testing.T) {
		if _, ok, err := period.SequenceDayOffsetAt(
			time.Date(2026, time.February, 7, 8, 0, 0, 0, time.UTC),
			entities.RegionOntario,
			holidays,
		); err != nil || ok {
			t.Fatalf("expected weekend to have no valid offset, got ok=%v err=%v", ok, err)
		}
	})

	t.Run("holiday is invalid", func(t *testing.T) {
		holidays := &holidaymock.HolidayChecker{}
		holidays.SetHoliday(time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC), entities.RegionOntario, true)

		if _, ok, err := period.SequenceDayOffsetAt(
			time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC),
			entities.RegionOntario,
			holidays,
		); err != nil || ok {
			t.Fatalf("expected holiday to have no valid offset, got ok=%v err=%v", ok, err)
		}
	})
}

func TestReminderScheduleMatchAt(t *testing.T) {
	holidays := &holidaymock.HolidayChecker{}
	schedule := entities.ReminderSchedule{
		PeriodType: entities.PeriodMonthly,
		Region:     entities.RegionOntario,
		Sequence:   entities.SequenceDayOffsets{0, 3, 5, 7},
	}

	t.Run("matching date returns sequence match", func(t *testing.T) {
		match, ok, err := schedule.MatchAt(time.Date(2026, time.February, 11, 8, 0, 0, 0, time.UTC), holidays)
		if err != nil {
			t.Fatalf("MatchAt returned error: %v", err)
		}
		if !ok {
			t.Fatalf("expected schedule match")
		}
		if match.Period.ID != "2026-02" || match.SequenceDayOffset != 7 || match.SequenceIndex != 3 {
			t.Fatalf("unexpected match: %+v", match)
		}
	})

	t.Run("non sequence business day returns false", func(t *testing.T) {
		if _, ok, err := schedule.MatchAt(time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC), holidays); err != nil || ok {
			t.Fatalf("expected no match, got ok=%v err=%v", ok, err)
		}
	})

	t.Run("pre sequence start date returns false", func(t *testing.T) {
		weekly := entities.ReminderSchedule{
			PeriodType: entities.PeriodWeekly,
			Region:     entities.RegionOntario,
			Sequence:   entities.SequenceDayOffsets{0},
		}

		if _, ok, err := weekly.MatchAt(time.Date(2026, time.February, 8, 8, 0, 0, 0, time.UTC), holidays); err != nil || ok {
			t.Fatalf("expected no match before sequence start, got ok=%v err=%v", ok, err)
		}
	})
}
