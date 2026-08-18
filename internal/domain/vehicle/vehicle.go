package vehicle

import (
	"time"

	"sanitation-operations/internal/apperror"
)

type Status string

const (
	Available   Status = "available"
	OnDuty      Status = "on_duty"
	Maintenance Status = "maintenance"
	Retired     Status = "retired"
)

type Vehicle struct {
	ID              string    `json:"id"`
	PlateNumber     string    `json:"plate_number"`
	VehicleType     string    `json:"vehicle_type"`
	DepotCode       string    `json:"depot_code"`
	Status          Status    `json:"status"`
	CapacityKg      int       `json:"capacity_kg"`
	OdometerKm      int       `json:"odometer_km"`
	InspectionDueAt time.Time `json:"inspection_due_at"`
	Version         int       `json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func New(id, plate, kind, depot string, capacity, odometer int, due, now time.Time) (Vehicle, error) {
	if id == "" || plate == "" || kind == "" || depot == "" || capacity <= 0 || odometer < 0 {
		return Vehicle{}, apperror.Validation(apperror.ErrValidation)
	}
	return Vehicle{ID: id, PlateNumber: plate, VehicleType: kind, DepotCode: depot, Status: Available, CapacityKg: capacity, OdometerKm: odometer, InspectionDueAt: due.UTC(), Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (v Vehicle) CanDispatch(at time.Time) error {
	if v.Status != Available {
		return apperror.Conflict(apperror.ErrUnavailable)
	}
	if !at.Before(v.InspectionDueAt) {
		return apperror.Conflict(apperror.ErrValidation)
	}
	return nil
}

func (v Vehicle) StartDispatch(at time.Time) (Vehicle, error) {
	if err := v.CanDispatch(at); err != nil {
		return Vehicle{}, err
	}
	v.Status = OnDuty
	v.Version++
	v.UpdatedAt = at.UTC()
	return v, nil
}

func (v Vehicle) FinishReturn(odometer int, at time.Time) (Vehicle, error) {
	if v.Status != OnDuty || odometer < v.OdometerKm {
		return Vehicle{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	v.Status = Available
	v.OdometerKm = odometer
	v.Version++
	v.UpdatedAt = at.UTC()
	return v, nil
}

func (v Vehicle) MarkMaintenance(at time.Time) (Vehicle, error) {
	if v.Status == Retired || v.Status == OnDuty {
		return Vehicle{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	v.Status = Maintenance
	v.Version++
	v.UpdatedAt = at.UTC()
	return v, nil
}

func (v Vehicle) ReleaseMaintenance(at time.Time) (Vehicle, error) {
	if v.Status != Maintenance {
		return Vehicle{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	v.Status = Available
	v.Version++
	v.UpdatedAt = at.UTC()
	return v, nil
}
