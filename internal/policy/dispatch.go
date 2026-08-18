package policy

import (
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/domain/inspection"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/domain/workplan"
)

type AssignmentFacts struct {
	Vehicle           vehicle.Vehicle
	Route             workplan.Route
	Shift             workplan.Shift
	ActiveMaintenance bool
	LatestInspection  *inspection.Inspection
	At                time.Time
}

func CheckAssignment(f AssignmentFacts) error {
	if f.Route.Status != workplan.RouteActive || f.Shift.Status != workplan.Scheduled {
		return apperror.Conflict(apperror.ErrInvalidState)
	}
	if err := f.Vehicle.CanDispatch(f.At); err != nil {
		return err
	}
	if f.ActiveMaintenance || f.Vehicle.CapacityKg < f.Route.RequiredCapacityKg {
		return apperror.Conflict(apperror.ErrUnavailable)
	}
	if f.LatestInspection != nil {
		if f.LatestInspection.Status != inspection.Passed || !f.Shift.StartAt.Before(f.LatestInspection.ExpiresAt) {
			return apperror.Conflict(apperror.ErrUnavailable)
		}
	}
	return nil
}

type StartFacts struct {
	Vehicle           vehicle.Vehicle
	Route             workplan.Route
	Shift             workplan.Shift
	StartOdometer     int
	LoadKg            int
	ActiveMaintenance bool
	At                time.Time
}

func CheckStart(f StartFacts) error {
	if f.Shift.Status != workplan.Assigned || f.Shift.AssignedVehicleID == nil || *f.Shift.AssignedVehicleID != f.Vehicle.ID {
		return apperror.Conflict(apperror.ErrInvalidState)
	}
	if err := f.Vehicle.CanDispatch(f.At); err != nil {
		return err
	}
	if f.ActiveMaintenance || f.Vehicle.CapacityKg < f.Route.RequiredCapacityKg {
		return apperror.Conflict(apperror.ErrUnavailable)
	}
	if f.StartOdometer < f.Vehicle.OdometerKm || f.LoadKg < 0 || f.LoadKg > f.Vehicle.CapacityKg {
		return apperror.Conflict(apperror.ErrUnavailable)
	}
	return nil
}
