package query

import (
	"context"
	"time"

	"sanitation-operations/internal/domain/incident"
	"sanitation-operations/internal/domain/inspection"
	"sanitation-operations/internal/domain/maintenance"
	"sanitation-operations/internal/domain/trip"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/domain/workplan"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
)

type Service struct{ Store repository.Store }

func (s Service) Vehicles(ctx context.Context, filter repository.VehicleFilter, page pagination.Query) (pagination.Result[vehicle.Vehicle], error) {
	return s.Store.ListVehicles(ctx, filter, page)
}
func (s Service) Shifts(ctx context.Context, filter repository.ShiftFilter, page pagination.Query) (pagination.Result[workplan.Shift], error) {
	return s.Store.ListShifts(ctx, filter, page)
}
func (s Service) Trips(ctx context.Context, filter repository.TripFilter, page pagination.Query) (pagination.Result[trip.Trip], error) {
	return s.Store.ListTrips(ctx, filter, page)
}
func (s Service) Maintenance(ctx context.Context, vehicleID string, page pagination.Query) (pagination.Result[maintenance.Order], error) {
	return s.Store.ListMaintenance(ctx, vehicleID, page)
}
func (s Service) Incidents(ctx context.Context, page pagination.Query) (pagination.Result[incident.Incident], error) {
	return s.Store.ListIncidents(ctx, page)
}
func (s Service) Inspections(ctx context.Context, vehicleID string, page pagination.Query) (pagination.Result[inspection.Inspection], error) {
	return s.Store.ListInspections(ctx, vehicleID, page)
}
func DateFilter(from, to time.Time) repository.TripFilter {
	return repository.TripFilter{From: &from, To: &to}
}
