package entities

import "time"

func normalizeDate(at time.Time) time.Time {
	year, month, day := at.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func isBusinessWeekday(day time.Time) bool {
	return day.Weekday() >= time.Monday && day.Weekday() <= time.Friday
}

func isHoliday(day time.Time, region ClientRegion, holidays HolidayChecker) (bool, error) {
	if holidays == nil {
		return false, nil
	}
	return holidays.IsHoliday(day, region)
}
