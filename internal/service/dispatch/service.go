package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/audit"
	"sanitation-operations/internal/clock"
	"sanitation-operations/internal/domain/incident"
	"sanitation-operations/internal/domain/inspection"
	"sanitation-operations/internal/domain/trip"
	"sanitation-operations/internal/domain/workplan"
	"sanitation-operations/internal/idempotency"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/policy"
	"sanitation-operations/internal/repository"
	"sanitation-operations/internal/service/scheduling"
)

type Service struct {
	Store          repository.Store
	Clock          clock.Clock
	IDs            identity.Generator
	IdempotencyTTL time.Duration
	Scheduler      scheduling.Checker
}

type AssignInput struct{ ShiftID, VehicleID, ActorID, RequestID string }
type StartTripInput struct {
	VehicleID, ShiftID, DriverID, DriverName, IdempotencyKey, ActorID, RequestID string
	StartOdometer, LoadKg                                                        int
}
type ReturnTripInput struct {
	TripID, ActorID, RequestID string
	EndOdometer                int
}
type IncidentInput struct {
	TripID                      string
	Severity                    incident.Severity
	Summary, ActorID, RequestID string
	OccurredAt                  time.Time
}

func (s Service) AssignShift(ctx context.Context, input AssignInput) (workplan.Shift, error) {
	if err := contextError(ctx); err != nil {
		return workplan.Shift{}, err
	}
	var result workplan.Shift
	err := s.Store.WithTx(ctx, func(ctx context.Context, tx repository.Tx) error {
		shift, err := tx.GetShift(ctx, input.ShiftID)
		if err != nil {
			return err
		}
		route, err := tx.GetRoute(ctx, shift.RouteID)
		if err != nil {
			return err
		}
		vehicleItem, err := tx.GetVehicle(ctx, input.VehicleID)
		if err != nil {
			return err
		}
		if err := s.Scheduler.CanAssign(ctx, tx, shift, vehicleItem.ID); err != nil {
			return err
		}
		_, activeMaintenance, err := tx.ActiveMaintenanceForVehicle(ctx, vehicleItem.ID)
		if err != nil {
			return err
		}
		latest, hasInspection, err := tx.LatestInspectionForVehicle(ctx, vehicleItem.ID)
		if err != nil {
			return err
		}
		var inspectionPtr *inspection.Inspection
		if hasInspection {
			inspectionPtr = &latest
		}
		if err := policy.CheckAssignment(policy.AssignmentFacts{Vehicle: vehicleItem, Route: route, Shift: shift, ActiveMaintenance: activeMaintenance, LatestInspection: inspectionPtr, At: s.Clock.Now()}); err != nil {
			return err
		}
		previous := shift
		result, err = shift.Assign(vehicleItem.ID, s.Clock.Now())
		if err != nil {
			return err
		}
		if err := tx.SaveShift(ctx, result, previous.Version); err != nil {
			return err
		}
		return tx.AppendAudit(ctx, s.event(input.ActorID, input.RequestID, "shift", result.ID, "assign", "success", map[string]any{"vehicle_id": vehicleItem.ID}))
	})
	return result, apperror.Wrap("assign shift", err)
}

func (s Service) StartTrip(ctx context.Context, input StartTripInput) (trip.Trip, error) {
	if err := contextError(ctx); err != nil {
		return trip.Trip{}, err
	}
	if input.IdempotencyKey == "" {
		return trip.Trip{}, apperror.Validation(fmt.Errorf("idempotency key is required"))
	}
	if existing, found, err := s.Store.FindTripByKey(ctx, input.VehicleID, input.IdempotencyKey); err != nil {
		return trip.Trip{}, apperror.Wrap("find idempotent trip", err)
	} else if found {
		return existing, nil
	}
	var result trip.Trip
	err := s.Store.WithTx(ctx, func(ctx context.Context, tx repository.Tx) error {
		now := s.Clock.Now()
		shift, err := tx.GetShift(ctx, input.ShiftID)
		if err != nil {
			return err
		}
		route, err := tx.GetRoute(ctx, shift.RouteID)
		if err != nil {
			return err
		}
		vehicleItem, err := tx.GetVehicle(ctx, input.VehicleID)
		if err != nil {
			return err
		}
		_, activeMaintenance, err := tx.ActiveMaintenanceForVehicle(ctx, vehicleItem.ID)
		if err != nil {
			return err
		}
		if err := policy.CheckStart(policy.StartFacts{Vehicle: vehicleItem, Route: route, Shift: shift, StartOdometer: input.StartOdometer, LoadKg: input.LoadKg, ActiveMaintenance: activeMaintenance, At: now}); err != nil {
			return err
		}
		driver, err := tx.GetDriver(ctx, input.DriverID)
		if err != nil {
			return err
		}
		if err := driver.CanOperate(vehicleItem.VehicleType, now); err != nil {
			return err
		}
		previousVehicle := vehicleItem
		updatedVehicle, err := vehicleItem.StartDispatch(now)
		if err != nil {
			return err
		}
		if err := tx.SaveVehicle(ctx, updatedVehicle, previousVehicle.Version); err != nil {
			return err
		}
		previousShift := shift
		shift, err = shift.Start(now)
		if err != nil {
			return err
		}
		if err := tx.SaveShift(ctx, shift, previousShift.Version); err != nil {
			return err
		}
		result, err = trip.New(s.IDs.NewID("trip"), vehicleItem.ID, shift.ID, driver.ID, driver.Name, input.IdempotencyKey, now)
		if err != nil {
			return err
		}
		result, err = result.Start(input.StartOdometer, input.LoadKg, now)
		if err != nil {
			return err
		}
		if err := tx.SaveTrip(ctx, result, 0); err != nil {
			return err
		}
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		expiry := now.Add(s.ttl())
		if err := tx.SaveIdempotency(ctx, idempotency.Record{Scope: "start-trip", Key: input.VehicleID + ":" + input.IdempotencyKey, Status: 201, Response: data, CreatedAt: now, ExpiresAt: expiry}); err != nil {
			return err
		}
		if _, err := tx.Enqueue(ctx, "trip.started", data, now); err != nil {
			return err
		}
		return tx.AppendAudit(ctx, s.event(input.ActorID, input.RequestID, "trip", result.ID, "start", "success", map[string]any{"shift_id": shift.ID, "load_kg": input.LoadKg}))
	})
	return result, apperror.Wrap("start trip", err)
}

func (s Service) ReturnTrip(ctx context.Context, input ReturnTripInput) (trip.Trip, error) {
	if err := contextError(ctx); err != nil {
		return trip.Trip{}, err
	}
	var result trip.Trip
	err := s.Store.WithTx(ctx, func(ctx context.Context, tx repository.Tx) error {
		now := s.Clock.Now()
		current, err := tx.GetTrip(ctx, input.TripID)
		if err != nil {
			return err
		}
		vehicleItem, err := tx.GetVehicle(ctx, current.VehicleID)
		if err != nil {
			return err
		}
		shift, err := tx.GetShift(ctx, current.ShiftID)
		if err != nil {
			return err
		}
		result, err = current.Complete(input.EndOdometer, now)
		if err != nil {
			return err
		}
		if err := tx.SaveTrip(ctx, result, current.Version); err != nil {
			return err
		}
		previousVehicle := vehicleItem
		updated, err := vehicleItem.FinishReturn(input.EndOdometer, now)
		if err != nil {
			return err
		}
		if err := tx.SaveVehicle(ctx, updated, previousVehicle.Version); err != nil {
			return err
		}
		previousShift := shift
		shift, err = shift.Complete(now)
		if err != nil {
			return err
		}
		if err := tx.SaveShift(ctx, shift, previousShift.Version); err != nil {
			return err
		}
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err := tx.Enqueue(ctx, "trip.completed", data, now); err != nil {
			return err
		}
		return tx.AppendAudit(ctx, s.event(input.ActorID, input.RequestID, "trip", result.ID, "return", "success", map[string]any{"distance_km": result.Distance()}))
	})
	return result, apperror.Wrap("return trip", err)
}

func (s Service) ReportIncident(ctx context.Context, input IncidentInput) (incident.Incident, error) {
	if err := contextError(ctx); err != nil {
		return incident.Incident{}, err
	}
	var result incident.Incident
	err := s.Store.WithTx(ctx, func(ctx context.Context, tx repository.Tx) error {
		current, err := tx.GetTrip(ctx, input.TripID)
		if err != nil {
			return err
		}
		if current.Status != trip.Active && current.Status != trip.Completed {
			return apperror.Conflict(apperror.ErrInvalidState)
		}
		now := s.Clock.Now()
		occurred := input.OccurredAt
		if occurred.IsZero() {
			occurred = now
		}
		result, err = incident.New(s.IDs.NewID("incident"), current.ID, current.VehicleID, input.Severity, input.Summary, occurred, now)
		if err != nil {
			return err
		}
		if err := tx.SaveIncident(ctx, result); err != nil {
			return err
		}
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err := tx.Enqueue(ctx, "incident.reported", data, now); err != nil {
			return err
		}
		return tx.AppendAudit(ctx, s.event(input.ActorID, input.RequestID, "incident", result.ID, "report", "success", map[string]any{"severity": input.Severity}))
	})
	return result, apperror.Wrap("report incident", err)
}

func (s Service) event(actor, request, entity, id, action, result string, metadata map[string]any) audit.Event {
	return audit.Event{ID: s.IDs.NewID("audit"), ActorID: actor, EntityType: entity, EntityID: id, Action: action, Result: result, RequestID: request, Metadata: metadata, CreatedAt: s.Clock.Now()}
}
func (s Service) ttl() time.Duration {
	if s.IdempotencyTTL <= 0 {
		return 24 * time.Hour
	}
	return s.IdempotencyTTL
}
func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return apperror.Wrap("context", ctx.Err())
	default:
		return nil
	}
}
