package clock

import "time"

type Clock interface {
	Now() time.Time
}

type Real struct{}

func (Real) Now() time.Time { return time.Now().UTC() }

type Fixed struct{ Current time.Time }

func (f Fixed) Now() time.Time { return f.Current.UTC() }

func DayBounds(now time.Time, location *time.Location) (time.Time, time.Time) {
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return start.UTC(), start.Add(24 * time.Hour).UTC()
}

func WithinWindow(value, start, end time.Time) bool {
	return !value.Before(start) && value.Before(end)
}
