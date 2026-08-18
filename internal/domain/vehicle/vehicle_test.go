package vehicle_test

import (
	"errors"
	"testing"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/domain/vehicle"
)

func newVehicle(t *testing.T) vehicle.Vehicle {
	t.Helper()
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	value, err := vehicle.New("vehicle-1", "沪环-100", "compactor", "D1", 9000, 1200, now.Add(24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestNewVehicleValidatesRequiredBusinessFields(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name                   string
		id, plate, kind, depot string
		capacity, odometer     int
	}{{"missing id", "", "P", "kind", "D", 1, 0}, {"missing plate", "id", "", "kind", "D", 1, 0}, {"missing type", "id", "P", "", "D", 1, 0}, {"missing depot", "id", "P", "kind", "", 1, 0}, {"zero capacity", "id", "P", "kind", "D", 0, 0}, {"negative odometer", "id", "P", "kind", "D", 1, -1}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := vehicle.New(tc.id, tc.plate, tc.kind, tc.depot, tc.capacity, tc.odometer, now.Add(time.Hour), now)
			if err == nil {
				t.Fatal("expected validation error")
			}
			var app *apperror.AppError
			if !errors.As(err, &app) || app.Code != apperror.CodeValidation {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestVehicleDispatchAndReturnLifecycle(t *testing.T) {
	value := newVehicle(t)
	now := value.CreatedAt.Add(time.Hour)
	onDuty, err := value.StartDispatch(now)
	if err != nil {
		t.Fatal(err)
	}
	if onDuty.Status != vehicle.OnDuty || onDuty.Version != 2 {
		t.Fatalf("unexpected dispatched vehicle: %+v", onDuty)
	}
	if value.Status != vehicle.Available {
		t.Fatal("value receiver mutated original")
	}
	returned, err := onDuty.FinishReturn(1250, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if returned.Status != vehicle.Available || returned.OdometerKm != 1250 || returned.Version != 3 {
		t.Fatalf("unexpected returned vehicle: %+v", returned)
	}
}

func TestVehicleCannotDispatchAfterInspectionExpires(t *testing.T) {
	value := newVehicle(t)
	_, err := value.StartDispatch(value.InspectionDueAt)
	if err == nil {
		t.Fatal("expected expired inspection conflict")
	}
	var app *apperror.AppError
	if !errors.As(err, &app) || app.Status != 409 {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestVehicleCannotReturnWithLowerOdometer(t *testing.T) {
	value := newVehicle(t)
	onDuty, _ := value.StartDispatch(value.CreatedAt.Add(time.Hour))
	_, err := onDuty.FinishReturn(value.OdometerKm-1, value.CreatedAt.Add(2*time.Hour))
	if err == nil {
		t.Fatal("expected odometer conflict")
	}
}
func TestVehicleMaintenanceLifecycle(t *testing.T) {
	value := newVehicle(t)
	blocked, err := value.MarkMaintenance(value.CreatedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Status != vehicle.Maintenance {
		t.Fatalf("status=%s", blocked.Status)
	}
	available, err := blocked.ReleaseMaintenance(value.CreatedAt.Add(2 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if available.Status != vehicle.Available {
		t.Fatalf("status=%s", available.Status)
	}
}
func TestOnDutyVehicleCannotEnterMaintenance(t *testing.T) {
	value := newVehicle(t)
	onDuty, _ := value.StartDispatch(value.CreatedAt.Add(time.Hour))
	if _, err := onDuty.MarkMaintenance(value.CreatedAt.Add(2 * time.Hour)); err == nil {
		t.Fatal("expected conflict")
	}
}
