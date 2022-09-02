package sakeyrotator

import "time"

func startOfDay() time.Time {
	t := time.Now().AddDate(0, 0, 0)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
