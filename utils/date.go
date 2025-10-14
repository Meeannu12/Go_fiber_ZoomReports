package utils

import "time"

// ParseDateRange parses fromDate and toDate query params
func ParseDateRange(fromStr, toStr string) (time.Time, time.Time, error) {
	var fromDate, toDate time.Time
	var err error

	if fromStr != "" {
		fromDate, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	} else {
		fromDate = time.Now().UTC()
	}

	if toStr != "" {
		toDate, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	} else {
		toDate = time.Now().UTC()
	}

	startOfDay := time.Date(fromDate.Year(), fromDate.Month(), fromDate.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 23, 59, 59, 999000000, time.UTC)

	return startOfDay, endOfDay, nil
}
