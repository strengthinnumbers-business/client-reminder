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
