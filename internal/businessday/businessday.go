package businessday

import (
	"fmt"
	"time"
)

type Calendar struct {
	Location   *time.Location
	CutoffHour int
}

func New(location string, cutoffHour int) (Calendar, error) {
	zone, err := time.LoadLocation(location)
	if err != nil {
		return Calendar{}, fmt.Errorf("load business timezone: %w", err)
	}
	if cutoffHour < 0 || cutoffHour > 23 {
		return Calendar{}, fmt.Errorf("cutoff hour must be between 0 and 23")
	}
	return Calendar{Location: zone, CutoffHour: cutoffHour}, nil
}

func (c Calendar) ServiceDate(value time.Time) string {
	local := value.In(c.Location)
	if local.Hour() < c.CutoffHour {
		local = local.AddDate(0, 0, -1)
	}
	return local.Format("2006-01-02")
}
func (c Calendar) Bounds(serviceDate string) (time.Time, time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", serviceDate, c.Location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse service date: %w", err)
	}
	start := day.Add(time.Duration(c.CutoffHour) * time.Hour)
	return start.UTC(), start.Add(24 * time.Hour).UTC(), nil
}
func (c Calendar) ValidateWindow(serviceDate string, start, end time.Time) error {
	lower, upper, err := c.Bounds(serviceDate)
	if err != nil {
		return err
	}
	if start.Before(lower) || !start.Before(upper) {
		return fmt.Errorf("shift start is outside service day")
	}
	if !end.After(start) || end.After(upper) {
		return fmt.Errorf("shift end is outside service day")
	}
	return nil
}
func (c Calendar) ParseLocal(serviceDate, clock string) (time.Time, error) {
	value, err := time.ParseInLocation("2006-01-02 15:04", serviceDate+" "+clock, c.Location)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse local service time: %w", err)
	}
	return value.UTC(), nil
}
func (c Calendar) Next(serviceDate string) (string, error) {
	day, err := time.ParseInLocation("2006-01-02", serviceDate, c.Location)
	if err != nil {
		return "", err
	}
	return day.AddDate(0, 0, 1).Format("2006-01-02"), nil
}
