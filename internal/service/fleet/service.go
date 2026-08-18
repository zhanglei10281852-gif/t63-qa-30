package fleet

import (
	"context"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/audit"
	"sanitation-operations/internal/clock"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/repository"
	"sanitation-operations/internal/validation"
)

type Service struct {
	Store repository.Store
	Clock clock.Clock
	IDs   identity.Generator
}
type CreateInput struct {
	PlateNumber, VehicleType, DepotCode string
	CapacityKg, OdometerKm              int
	InspectionDueAt                     time.Time
	ActorID, RequestID                  string
}

func (s Service) Create(ctx context.Context, input CreateInput) (vehicle.Vehicle, error) {
	input.PlateNumber = validation.NormalizePlate(input.PlateNumber)
	var checks validation.Collector
	checks.Required("plate_number", input.PlateNumber)
	checks.Plate("plate_number", input.PlateNumber)
	checks.Required("vehicle_type", input.VehicleType)
	checks.Required("depot_code", input.DepotCode)
	checks.Positive("capacity_kg", input.CapacityKg)
	checks.NonNegative("odometer_km", input.OdometerKm)
	checks.Future("inspection_due_at", input.InspectionDueAt, s.Clock.Now())
	if err := checks.Err(); err != nil {
		return vehicle.Vehicle{}, apperror.Validation(err)
	}
	now := s.Clock.Now()
	item, err := vehicle.New(s.IDs.NewID("vehicle"), input.PlateNumber, input.VehicleType, input.DepotCode, input.CapacityKg, input.OdometerKm, input.InspectionDueAt, now)
	if err != nil {
		return vehicle.Vehicle{}, err
	}
	if err := s.Store.SaveVehicle(ctx, item, 0); err != nil {
		return vehicle.Vehicle{}, apperror.Wrap("save vehicle", err)
	}
	if err := s.Store.AppendAudit(ctx, audit.Event{ID: s.IDs.NewID("audit"), ActorID: input.ActorID, EntityType: "vehicle", EntityID: item.ID, Action: "create", Result: "success", RequestID: input.RequestID, Metadata: map[string]any{"plate_number": item.PlateNumber}, CreatedAt: now}); err != nil {
		return vehicle.Vehicle{}, err
	}
	return item, nil
}
