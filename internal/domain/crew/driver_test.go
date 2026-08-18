package crew_test

import (
	"testing"
	"time"

	"sanitation-operations/internal/domain/crew"
)

var current = time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)

func driver(t *testing.T) crew.Driver {
	t.Helper()
	value, err := crew.New("driver-1", "DRV-1", "Lin", "B2", current.AddDate(1, 0, 0), current)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestDriverRequiresActiveLicense(t *testing.T) {
	if _, err := crew.New("id", "NO", "name", "B2", current, current); err == nil {
		t.Fatal("expected expired license error")
	}
}
func TestCertificationAllowsMatchingVehicleType(t *testing.T) {
	value := driver(t)
	updated, err := value.AddCertification(crew.Certification{ID: "cert", DriverID: value.ID, Code: "COMP", VehicleType: "compactor", ExpiresAt: current.AddDate(0, 6, 0)}, current)
	if err != nil {
		t.Fatal(err)
	}
	if err := updated.CanOperate("compactor", current.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := updated.CanOperate("sweeper", current.Add(time.Hour)); err == nil {
		t.Fatal("unexpected qualification")
	}
}
func TestExpiredCertificationCannotAuthorizeTrip(t *testing.T) {
	value := driver(t)
	value.Certifications = []crew.Certification{{ID: "cert", DriverID: value.ID, Code: "COMP", VehicleType: "compactor", ExpiresAt: current.Add(-time.Minute)}}
	if err := value.CanOperate("compactor", current); err == nil {
		t.Fatal("expected certification expiry conflict")
	}
}
func TestSuspendedDriverCannotOperate(t *testing.T) {
	value := driver(t)
	value, _ = value.AddCertification(crew.Certification{ID: "cert", DriverID: value.ID, Code: "COMP", VehicleType: "compactor", ExpiresAt: current.AddDate(1, 0, 0)}, current)
	suspended, err := value.Suspend(current.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := suspended.CanOperate("compactor", current.Add(2*time.Hour)); err == nil {
		t.Fatal("suspended driver operated")
	}
	active, err := suspended.Reactivate(current.Add(3 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if active.Status != crew.Active {
		t.Fatalf("status=%s", active.Status)
	}
}
func TestCertificationReplacementDoesNotShareSlice(t *testing.T) {
	value := driver(t)
	value, _ = value.AddCertification(crew.Certification{ID: "one", DriverID: value.ID, Code: "A", VehicleType: "compactor", ExpiresAt: current.AddDate(1, 0, 0)}, current)
	copy := value.Clone()
	copy.Certifications[0].VehicleType = "sweeper"
	if value.Certifications[0].VehicleType != "compactor" {
		t.Fatal("clone polluted original")
	}
	replacement := crew.Certification{ID: "two", DriverID: value.ID, Code: "A", VehicleType: "sweeper", ExpiresAt: current.AddDate(1, 0, 0)}
	value, _ = value.AddCertification(replacement, current)
	if len(value.Certifications) != 1 || value.Certifications[0].ID != "two" {
		t.Fatalf("unexpected certifications: %+v", value.Certifications)
	}
}
