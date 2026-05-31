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

func AddBusinessDays(from time.Time, days int, region ClientRegion, holidays HolidayChecker) (time.Time, error) {
	current := normalizeDate(from)
	if days <= 0 {
		return current, nil
	}

	counted := 0
	for counted < days {
		current = current.AddDate(0, 0, 1)
		if !isBusinessWeekday(current) {
			continue
		}

		holiday, err := isHoliday(current, region, holidays)
		if err != nil {
			return time.Time{}, err
		}
		if holiday {
			continue
		}

		counted++
	}

	return current, nil
}
