package repository

import (
	"context"
	"time"

	"sanitation-operations/internal/audit"
	"sanitation-operations/internal/domain/crew"
	"sanitation-operations/internal/domain/fuel"
	"sanitation-operations/internal/domain/incident"
	"sanitation-operations/internal/domain/inspection"
	"sanitation-operations/internal/domain/maintenance"
	"sanitation-operations/internal/domain/operator"
	"sanitation-operations/internal/domain/trip"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/domain/workplan"
	"sanitation-operations/internal/idempotency"
	"sanitation-operations/internal/pagination"
)

type VehicleFilter struct {
	Status vehicle.Status
	Depot  string
	Query  string
}
type ShiftFilter struct {
	ServiceDate string
	Status      workplan.ShiftStatus
	RouteID     string
}
type TripFilter struct {
	Status    trip.Status
	VehicleID string
	From, To  *time.Time
}

type Store interface {
	WithTx(context.Context, func(context.Context, Tx) error) error
	VehicleReader
	VehicleWriter
	ShiftReader
	ShiftWriter
	TripReader
	TripWriter
	MaintenanceReader
	MaintenanceWriter
	IncidentReader
	IncidentWriter
	InspectionReader
	InspectionWriter
	FuelReader
	FuelWriter
	DriverReader
	DriverWriter
	OperatorReader
	OperatorWriter
	RouteReader
	RouteWriter
	AuditWriter
	IdempotencyReader
	OutboxStore
}

type Tx interface {
	VehicleReader
	VehicleWriter
	ShiftReader
	ShiftWriter
	TripReader
	TripWriter
	MaintenanceReader
	MaintenanceWriter
	IncidentReader
	IncidentWriter
	InspectionReader
	InspectionWriter
	FuelReader
	FuelWriter
	DriverReader
	DriverWriter
	OperatorReader
	OperatorWriter
	RouteReader
	RouteWriter
	AuditWriter
	IdempotencyReader
	OutboxStore
}

type VehicleReader interface {
	GetVehicle(context.Context, string) (vehicle.Vehicle, error)
	ListVehicles(context.Context, VehicleFilter, pagination.Query) (pagination.Result[vehicle.Vehicle], error)
}
type VehicleWriter interface {
	SaveVehicle(context.Context, vehicle.Vehicle, int) error
}
type RouteReader interface {
	GetRoute(context.Context, string) (workplan.Route, error)
	ListRoutes(context.Context, pagination.Query) (pagination.Result[workplan.Route], error)
}
type RouteWriter interface {
	SaveRoute(context.Context, workplan.Route) error
}
type ShiftReader interface {
	GetShift(context.Context, string) (workplan.Shift, error)
	ListShifts(context.Context, ShiftFilter, pagination.Query) (pagination.Result[workplan.Shift], error)
}
type ShiftWriter interface {
	SaveShift(context.Context, workplan.Shift, int) error
}
type TripReader interface {
	GetTrip(context.Context, string) (trip.Trip, error)
	FindTripByKey(context.Context, string, string) (trip.Trip, bool, error)
	ListTrips(context.Context, TripFilter, pagination.Query) (pagination.Result[trip.Trip], error)
}
type TripWriter interface {
	SaveTrip(context.Context, trip.Trip, int) error
}
type MaintenanceReader interface {
	GetMaintenance(context.Context, string) (maintenance.Order, error)
	ActiveMaintenanceForVehicle(context.Context, string) (maintenance.Order, bool, error)
	ListMaintenance(context.Context, string, pagination.Query) (pagination.Result[maintenance.Order], error)
}
type MaintenanceWriter interface {
	SaveMaintenance(context.Context, maintenance.Order, int) error
}
type IncidentReader interface {
	GetIncident(context.Context, string) (incident.Incident, error)
	ListIncidents(context.Context, pagination.Query) (pagination.Result[incident.Incident], error)
}
type IncidentWriter interface {
	SaveIncident(context.Context, incident.Incident) error
}
type InspectionReader interface {
	GetInspection(context.Context, string) (inspection.Inspection, error)
	LatestInspectionForVehicle(context.Context, string) (inspection.Inspection, bool, error)
	ListInspections(context.Context, string, pagination.Query) (pagination.Result[inspection.Inspection], error)
}
type InspectionWriter interface {
	SaveInspection(context.Context, inspection.Inspection, int) error
}
type FuelReader interface {
	LatestFuel(context.Context, string) (fuel.Record, bool, error)
	ListFuel(context.Context, string, pagination.Query) (pagination.Result[fuel.Record], error)
}
type FuelWriter interface {
	SaveFuel(context.Context, fuel.Record) error
}
type DriverReader interface {
	GetDriver(context.Context, string) (crew.Driver, error)
	ListDrivers(context.Context, string, pagination.Query) (pagination.Result[crew.Driver], error)
}
type DriverWriter interface {
	SaveDriver(context.Context, crew.Driver, int) error
}
type OperatorReader interface {
	GetOperator(context.Context, string) (operator.Operator, error)
	GetOperatorByUsername(context.Context, string) (operator.Operator, error)
	GetSession(context.Context, string) (operator.Session, error)
}
type OperatorWriter interface {
	SaveOperator(context.Context, operator.Operator) error
	SaveSession(context.Context, operator.Session) error
	RevokeSession(context.Context, string, time.Time) error
}
type AuditWriter interface {
	AppendAudit(context.Context, audit.Event) error
}
type IdempotencyReader interface {
	FindIdempotency(context.Context, string, string, time.Time) (idempotency.Record, bool, error)
	SaveIdempotency(context.Context, idempotency.Record) error
}
type OutboxStore interface {
	Enqueue(context.Context, string, []byte, time.Time) (string, error)
	ClaimDue(context.Context, time.Time, int) ([]OutboxJob, error)
	MarkJobDone(context.Context, string) error
	MarkJobRetry(context.Context, string, string, time.Time) error
	MarkJobPermanent(context.Context, string, string) error
}

type OutboxJob struct {
	ID            string
	Type          string
	Payload       []byte
	Attempts      int
	NextAttemptAt time.Time
	LastError     string
}
