package entities

import (
	"fmt"
	"time"
)

type PeriodType int

const (
	PeriodWeekly PeriodType = iota
	PeriodMonthly
	PeriodQuarterly
)

type Period struct {
	Type PeriodType
	ID   string
}

func (p Period) Start() time.Time {
	switch p.Type {
	case PeriodWeekly:
		var year, isoWeek int
		if _, err := fmt.Sscanf(p.ID, "%d-W%02d", &year, &isoWeek); err == nil {
			return isoWeekStart(year, isoWeek)
		}
	case PeriodQuarterly:
		var year, quarter int
		if _, err := fmt.Sscanf(p.ID, "%d-Q%d", &year, &quarter); err == nil && quarter >= 1 && quarter <= 4 {
			quarterStartMonth := time.Month((quarter-1)*3 + 1)
			return time.Date(year, quarterStartMonth, 1, 0, 0, 0, 0, time.UTC)
		}
	case PeriodMonthly:
		fallthrough
	default:
		if start, err := time.Parse("2006-01", p.ID); err == nil {
			return time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
		}
	}

	return time.Time{}
}

func CurrentPeriod(periodType PeriodType, now time.Time) Period {
	now = now.UTC()

	switch periodType {
	case PeriodWeekly:
		year, isoWeek := now.ISOWeek()
		return Period{Type: periodType, ID: fmt.Sprintf("%d-W%02d", year, isoWeek)}
	case PeriodQuarterly:
		quarter := (int(now.Month())-1)/3 + 1
		return Period{Type: periodType, ID: fmt.Sprintf("%d-Q%d", now.Year(), quarter)}
	case PeriodMonthly:
		fallthrough
	default:
		return Period{Type: periodType, ID: now.Format("2006-01")}
	}
}

func (p Period) Previous() Period {
	start := p.Start()
	if start.IsZero() {
		return Period{Type: p.Type}
	}

	switch p.Type {
	case PeriodWeekly:
		return CurrentPeriod(p.Type, start.AddDate(0, 0, -1))
	case PeriodQuarterly:
		return CurrentPeriod(p.Type, start.AddDate(0, -1, 0))
	case PeriodMonthly:
		fallthrough
	default:
		return CurrentPeriod(p.Type, start.AddDate(0, 0, -1))
	}
}

func (p Period) FirstSequenceDay(region ClientRegion, holidays HolidayChecker) (time.Time, error) {
	day := p.Start()
	for day.Weekday() != time.Monday {
		day = day.AddDate(0, 0, 1)
	}

	for {
		if !isBusinessWeekday(day) {
			day = day.AddDate(0, 0, 1)
			continue
		}

		holiday, err := isHoliday(day, region, holidays)
		if err != nil {
			return time.Time{}, err
		}
		if !holiday {
			return day, nil
		}

		day = day.AddDate(0, 0, 1)
	}
}

func isoWeekStart(year, isoWeek int) time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekdayOffset := (int(jan4.Weekday()) + 6) % 7
	weekOneStart := jan4.AddDate(0, 0, -weekdayOffset)
	return weekOneStart.AddDate(0, 0, (isoWeek-1)*7)
}
