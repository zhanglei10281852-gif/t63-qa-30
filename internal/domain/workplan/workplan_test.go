package workplan_test

import (
	"testing"
	"time"

	"sanitation-operations/internal/domain/workplan"
)

var now = time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)

func shift(t *testing.T) workplan.Shift {
	t.Helper()
	value, err := workplan.NewShift("shift-1", "route-1", "2026-08-18", now.Add(time.Hour), now.Add(5*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestRouteRequiresPositiveCapacity(t *testing.T) {
	if _, err := workplan.NewRoute("route", "R-1", "route", "zone", 0, now); err == nil {
		t.Fatal("expected validation error")
	}
	route, err := workplan.NewRoute("route", "R-1", "route", "zone", 5000, now)
	if err != nil {
		t.Fatal(err)
	}
	if route.Status != workplan.RouteActive {
		t.Fatalf("status=%s", route.Status)
	}
}
func TestShiftAssignmentExecutionAndCompletion(t *testing.T) {
	value := shift(t)
	assigned, err := value.Assign("vehicle-1", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if assigned.Status != workplan.Assigned || assigned.AssignedVehicleID == nil {
		t.Fatalf("unexpected: %+v", assigned)
	}
	active, err := assigned.Start(now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	done, err := active.Complete(now.Add(5 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != workplan.Completed || done.Version != 4 {
		t.Fatalf("unexpected: %+v", done)
	}
}
func TestShiftRejectsInvalidTransitionOrder(t *testing.T) {
	value := shift(t)
	if _, err := value.Start(now); err == nil {
		t.Fatal("unassigned shift started")
	}
	if _, err := value.Complete(now); err == nil {
		t.Fatal("scheduled shift completed")
	}
	assigned, _ := value.Assign("vehicle", now)
	if _, err := assigned.Complete(now); err == nil {
		t.Fatal("assigned shift completed without starting")
	}
}
func TestShiftCancellationRules(t *testing.T) {
	value := shift(t)
	cancelled, err := value.Cancel(now)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != workplan.Cancelled {
		t.Fatalf("status=%s", cancelled.Status)
	}
	assigned, _ := value.Assign("vehicle", now)
	active, _ := assigned.Start(now)
	if _, err := active.Cancel(now); err == nil {
		t.Fatal("active shift cancelled")
	}
}
func TestShiftRequiresIncreasingTimeWindow(t *testing.T) {
	if _, err := workplan.NewShift("id", "route", "date", now, now, now); err == nil {
		t.Fatal("expected invalid window")
	}
	if _, err := workplan.NewShift("id", "route", "date", now, now.Add(-time.Hour), now); err == nil {
		t.Fatal("expected invalid window")
	}
}
