package trip

import (
	"time"

	"sanitation-operations/internal/apperror"
)

type Status string

const (
	Planned   Status = "planned"
	Active    Status = "active"
	Completed Status = "completed"
	Cancelled Status = "cancelled"
)

type Trip struct {
	ID             string     `json:"id"`
	VehicleID      string     `json:"vehicle_id"`
	ShiftID        string     `json:"shift_id"`
	DriverID       string     `json:"driver_id"`
	Status         Status     `json:"status"`
	DriverName     string     `json:"driver_name"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	StartOdometer  *int       `json:"start_odometer,omitempty"`
	EndOdometer    *int       `json:"end_odometer,omitempty"`
	LoadKg         int        `json:"load_kg"`
	IdempotencyKey string     `json:"idempotency_key"`
	Version        int        `json:"version"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func New(id, vehicleID, shiftID, driverID, driver, key string, now time.Time) (Trip, error) {
	if id == "" || vehicleID == "" || shiftID == "" || driverID == "" || driver == "" || key == "" {
		return Trip{}, apperror.Validation(apperror.ErrValidation)
	}
	return Trip{ID: id, VehicleID: vehicleID, ShiftID: shiftID, DriverID: driverID, Status: Planned, DriverName: driver, IdempotencyKey: key, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (t Trip) Start(odometer, load int, at time.Time) (Trip, error) {
	if t.Status != Planned || odometer < 0 || load < 0 {
		return Trip{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	stamp := at.UTC()
	t.Status = Active
	t.StartOdometer = &odometer
	t.LoadKg = load
	t.StartedAt = &stamp
	t.Version++
	t.UpdatedAt = stamp
	return t, nil
}

func (t Trip) Complete(odometer int, at time.Time) (Trip, error) {
	if t.Status != Active || t.StartOdometer == nil || odometer < *t.StartOdometer {
		return Trip{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	stamp := at.UTC()
	t.Status = Completed
	t.EndOdometer = &odometer
	t.EndedAt = &stamp
	t.Version++
	t.UpdatedAt = stamp
	return t, nil
}

func (t Trip) Cancel(at time.Time) (Trip, error) {
	if t.Status == Completed || t.Status == Cancelled {
		return Trip{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	t.Status = Cancelled
	t.Version++
	t.UpdatedAt = at.UTC()
	return t, nil
}

func (t Trip) Distance() int {
	if t.StartOdometer == nil || t.EndOdometer == nil {
		return 0
	}
	return *t.EndOdometer - *t.StartOdometer
}
