package util

import "time"

var weekdayMap = map[string]int{
	"sunday": 0, "saturday": 6, "friday": 5, "thursday": 4,
	"wednesday": 3, "tuesday": 2, "monday": 1,
	"воскресенье": 0, "суббота": 6, "пятница": 5, "четверг": 4,
	"среда": 3, "вторник": 2, "понедельник": 1,
}

func ResolveDate(date, relativeDate string) string {
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err == nil {
			return date
		}
	}
	now := time.Now()
	switch relativeDate {
	case "today":
		return now.Format("2006-01-02")
	case "tomorrow":
		return now.AddDate(0, 0, 1).Format("2006-01-02")
	case "":
		return ""
	}
	if wd, ok := weekdayMap[relativeDate]; ok {
		diff := (wd - int(now.Weekday()) + 7) % 7
		if diff == 0 {
			return now.Format("2006-01-02")
		}
		return now.AddDate(0, 0, diff).Format("2006-01-02")
	}
	return now.Format("2006-01-02")
}
