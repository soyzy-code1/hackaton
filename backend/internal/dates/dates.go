package dates

import (
	"time"
)

func TodayInTZ(tzName string) time.Time {
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func AddMonthClamped(d time.Time, _ string) time.Time {
	y, m, _ := d.Date()
	mi := int(m)
	if mi == 12 {
		y++
		mi = 1
	} else {
		mi++
	}
	lastDay := daysInMonth(y, time.Month(mi))
	day := d.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(y, time.Month(mi), day, 0, 0, 0, 0, time.UTC)
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func DaysLeft(dueDate time.Time, tzName string) int {
	today := TodayInTZ(tzName)
	due := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, time.UTC)
	return int(due.Sub(today).Hours() / 24)
}

func IsOverdue(dueDate time.Time, isPaid bool, tzName string) bool {
	if isPaid {
		return false
	}
	today := TodayInTZ(tzName)
	due := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, time.UTC)
	return due.Before(today)
}

func MonthBounds(tzName string) (time.Time, time.Time) {
	today := TodayInTZ(tzName)
	start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	y, m := today.Year(), today.Month()
	if m == 12 {
		end := time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC)
		return start, end
	}
	next := time.Date(y, m+1, 1, 0, 0, 0, 0, time.UTC)
	end := next.AddDate(0, 0, -1)
	return start, end
}

func ParseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}
