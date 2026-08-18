package schedule_test

import (
	"testing"
	"time"

	"sanitation-operations/internal/domain/schedule"
)

var day = time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)

func window(t *testing.T, id string, start, end int) schedule.Window {
	t.Helper()
	value, err := schedule.NewWindow(id, day.Add(time.Duration(start)*time.Hour), day.Add(time.Duration(end)*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestRosterAcceptsSeparatedShifts(t *testing.T) {
	roster := schedule.NewRoster("vehicle", "2026-08-18")
	var err error
	roster, err = roster.Add(window(t, "morning", 4, 9), 16*time.Hour, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	roster, err = roster.Add(window(t, "evening", 10, 15), 16*time.Hour, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if roster.Duty != 10*time.Hour || len(roster.Windows) != 2 {
		t.Fatalf("unexpected roster: %+v", roster)
	}
}
func TestRosterRejectsOverlap(t *testing.T) {
	roster := schedule.NewRoster("vehicle", "date")
	roster, _ = roster.Add(window(t, "first", 4, 9), 16*time.Hour, 0)
	if _, err := roster.Add(window(t, "second", 8, 12), 16*time.Hour, 0); err == nil {
		t.Fatal("expected overlap conflict")
	}
}
func TestRosterRejectsInsufficientBreak(t *testing.T) {
	roster := schedule.NewRoster("vehicle", "date")
	roster, _ = roster.Add(window(t, "first", 4, 9), 16*time.Hour, 30*time.Minute)
	second, _ := schedule.NewWindow("second", day.Add(9*time.Hour+10*time.Minute), day.Add(12*time.Hour))
	if _, err := roster.Add(second, 16*time.Hour, 30*time.Minute); err == nil {
		t.Fatal("expected break conflict")
	}
}
func TestRosterRejectsDutyLimit(t *testing.T) {
	roster := schedule.NewRoster("vehicle", "date")
	roster, _ = roster.Add(window(t, "first", 0, 8), 12*time.Hour, 0)
	if _, err := roster.Add(window(t, "second", 9, 14), 12*time.Hour, 0); err == nil {
		t.Fatal("expected duty limit")
	}
}
func TestWindowRejectsInvalidRange(t *testing.T) {
	if _, err := schedule.NewWindow("id", day, day); err == nil {
		t.Fatal("expected invalid range")
	}
	if _, err := schedule.NewWindow("", day, day.Add(time.Hour)); err == nil {
		t.Fatal("expected missing id")
	}
}
