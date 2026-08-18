package trip_test

import (
	"testing"
	"time"

	"sanitation-operations/internal/domain/trip"
)

func planned(t *testing.T) trip.Trip {
	t.Helper()
	value, err := trip.New("trip-1", "vehicle-1", "shift-1", "driver-1", "Lin", "key-1", time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestTripLifecycleCapturesDistanceAndTimestamps(t *testing.T) {
	value := planned(t)
	startedAt := value.CreatedAt.Add(time.Hour)
	active, err := value.Start(1000, 5000, startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if active.Status != trip.Active || active.StartedAt == nil || *active.StartOdometer != 1000 {
		t.Fatalf("unexpected active trip: %+v", active)
	}
	completed, err := active.Complete(1064, startedAt.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != trip.Completed || completed.Distance() != 64 || completed.EndedAt == nil {
		t.Fatalf("unexpected completed trip: %+v", completed)
	}
}
func TestPlannedTripCanBeCancelled(t *testing.T) {
	value := planned(t)
	cancelled, err := value.Cancel(value.CreatedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != trip.Cancelled || cancelled.Version != 2 {
		t.Fatalf("unexpected: %+v", cancelled)
	}
}
func TestCompletedTripCannotBeCancelled(t *testing.T) {
	value := planned(t)
	active, _ := value.Start(10, 20, value.CreatedAt.Add(time.Hour))
	done, _ := active.Complete(12, value.CreatedAt.Add(2*time.Hour))
	if _, err := done.Cancel(value.CreatedAt.Add(3 * time.Hour)); err == nil {
		t.Fatal("expected conflict")
	}
}
func TestTripRejectsInvalidStartValues(t *testing.T) {
	cases := []struct {
		name           string
		odometer, load int
	}{{"negative odometer", -1, 10}, {"negative load", 10, -1}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := planned(t).Start(tc.odometer, tc.load, time.Now()); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
func TestTripRejectsLowerEndOdometer(t *testing.T) {
	value := planned(t)
	active, _ := value.Start(100, 10, value.CreatedAt.Add(time.Hour))
	if _, err := active.Complete(99, value.CreatedAt.Add(2*time.Hour)); err == nil {
		t.Fatal("expected error")
	}
}
func TestDistanceIsZeroBeforeCompletion(t *testing.T) {
	value := planned(t)
	if value.Distance() != 0 {
		t.Fatalf("distance=%d", value.Distance())
	}
	active, _ := value.Start(100, 10, value.CreatedAt.Add(time.Hour))
	if active.Distance() != 0 {
		t.Fatalf("distance=%d", active.Distance())
	}
}
