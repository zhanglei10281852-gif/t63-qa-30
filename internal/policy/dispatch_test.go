package policy

import (
	"testing"
	"time"

	"sanitation-operations/internal/domain/inspection"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/domain/workplan"
)

func facts(t *testing.T) (vehicle.Vehicle, workplan.Route, workplan.Shift, time.Time) {
	t.Helper()
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	v, err := vehicle.New("v1", "沪环-001", "compactor", "H-01", 9000, 100, now.Add(24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	r, err := workplan.NewRoute("r1", "H-001", "北片区", "north", 5000, now)
	if err != nil {
		t.Fatal(err)
	}
	s, err := workplan.NewShift("s1", r.ID, "2026-08-18", now.Add(time.Hour), now.Add(3*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	return v, r, s, now
}

func TestCheckAssignmentAcceptsHealthyFacts(t *testing.T) {
	v, r, s, now := facts(t)
	passed := inspection.Inspection{Status: inspection.Passed, ExpiresAt: now.Add(2 * time.Hour)}
	if err := CheckAssignment(AssignmentFacts{Vehicle: v, Route: r, Shift: s, LatestInspection: &passed, At: now}); err != nil {
		t.Fatalf("healthy assignment rejected: %v", err)
	}
}

func TestCheckAssignmentRejectsCapacityMaintenanceAndInspection(t *testing.T) {
	v, r, s, now := facts(t)
	r.RequiredCapacityKg = 10000
	if err := CheckAssignment(AssignmentFacts{Vehicle: v, Route: r, Shift: s, At: now}); err == nil {
		t.Fatal("undersized vehicle accepted")
	}
	r.RequiredCapacityKg = 5000
	if err := CheckAssignment(AssignmentFacts{Vehicle: v, Route: r, Shift: s, ActiveMaintenance: true, At: now}); err == nil {
		t.Fatal("vehicle under maintenance accepted")
	}
	expired := inspection.Inspection{Status: inspection.Passed, ExpiresAt: now}
	if err := CheckAssignment(AssignmentFacts{Vehicle: v, Route: r, Shift: s, LatestInspection: &expired, At: now}); err == nil {
		t.Fatal("expired inspection accepted")
	}
	failed := inspection.Inspection{Status: inspection.Failed, ExpiresAt: now.Add(time.Hour)}
	if err := CheckAssignment(AssignmentFacts{Vehicle: v, Route: r, Shift: s, LatestInspection: &failed, At: now}); err == nil {
		t.Fatal("failed inspection accepted")
	}
}

func TestCheckStartEnforcesAssignmentCapacityAndLoadBounds(t *testing.T) {
	v, r, s, now := facts(t)
	if err := CheckStart(StartFacts{Vehicle: v, Route: r, Shift: s, LoadKg: 6000, At: now}); err == nil {
		t.Fatal("unassigned shift accepted")
	}
	assigned, err := s.Assign(v.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckStart(StartFacts{Vehicle: v, Route: r, Shift: assigned, StartOdometer: v.OdometerKm, LoadKg: 0, At: now}); err != nil {
		t.Fatalf("empty outbound vehicle rejected: %v", err)
	}
	undersized := v
	undersized.CapacityKg = r.RequiredCapacityKg - 1
	if err := CheckStart(StartFacts{Vehicle: undersized, Route: r, Shift: assigned, StartOdometer: v.OdometerKm, LoadKg: 0, At: now}); err == nil {
		t.Fatal("undersized assigned vehicle accepted")
	}
	if err := CheckStart(StartFacts{Vehicle: v, Route: r, Shift: assigned, StartOdometer: v.OdometerKm - 1, LoadKg: 0, At: now}); err == nil {
		t.Fatal("odometer rollback accepted")
	}
	if err := CheckStart(StartFacts{Vehicle: v, Route: r, Shift: assigned, StartOdometer: v.OdometerKm, LoadKg: 9001, At: now}); err == nil {
		t.Fatal("overloaded trip accepted")
	}
	if err := CheckStart(StartFacts{Vehicle: v, Route: r, Shift: assigned, StartOdometer: v.OdometerKm, LoadKg: 6000, ActiveMaintenance: true, At: now}); err == nil {
		t.Fatal("maintenance vehicle started")
	}
	if err := CheckStart(StartFacts{Vehicle: v, Route: r, Shift: assigned, StartOdometer: v.OdometerKm, LoadKg: 6000, At: now}); err != nil {
		t.Fatalf("valid start rejected: %v", err)
	}
}

func TestCheckAssignmentRejectsClosedRouteAndWrongStates(t *testing.T) {
	v, r, s, now := facts(t)
	r.Status = workplan.RouteClosed
	if err := CheckAssignment(AssignmentFacts{Vehicle: v, Route: r, Shift: s, At: now}); err == nil {
		t.Fatal("closed route accepted")
	}
	r.Status = workplan.RouteActive
	s, _ = s.Assign(v.ID, now)
	if err := CheckAssignment(AssignmentFacts{Vehicle: v, Route: r, Shift: s, At: now}); err == nil {
		t.Fatal("already assigned shift accepted")
	}
}
