package maintenance

import (
	"context"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/audit"
	"sanitation-operations/internal/clock"
	"sanitation-operations/internal/domain/maintenance"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/repository"
)

type Service struct {
	Store repository.Store
	Clock clock.Clock
	IDs   identity.Generator
}
type OpenInput struct {
	VehicleID, Kind, Notes, ActorID, RequestID string
	DueAt                                      time.Time
}

func (s Service) Open(ctx context.Context, input OpenInput) (maintenance.Order, error) {
	var result maintenance.Order
	err := s.Store.WithTx(ctx, func(ctx context.Context, tx repository.Tx) error {
		now := s.Clock.Now()
		current, err := tx.GetVehicle(ctx, input.VehicleID)
		if err != nil {
			return err
		}
		if current.Status == vehicle.Retired {
			return apperror.Conflict(apperror.ErrInvalidState)
		}
		if _, exists, err := tx.ActiveMaintenanceForVehicle(ctx, input.VehicleID); err != nil {
			return err
		} else if exists {
			return apperror.Conflict(apperror.ErrConflict)
		}
		previous := current
		current, err = current.MarkMaintenance(now)
		if err != nil {
			return err
		}
		if err := tx.SaveVehicle(ctx, current, previous.Version); err != nil {
			return err
		}
		result, err = maintenance.New(s.IDs.NewID("maint"), input.VehicleID, input.Kind, input.Notes, now, input.DueAt, now)
		if err != nil {
			return err
		}
		if err := tx.SaveMaintenance(ctx, result, 0); err != nil {
			return err
		}
		return tx.AppendAudit(ctx, audit.Event{ID: s.IDs.NewID("audit"), ActorID: input.ActorID, EntityType: "maintenance", EntityID: result.ID, Action: "open", Result: "success", RequestID: input.RequestID, Metadata: map[string]any{"vehicle_id": input.VehicleID}, CreatedAt: now})
	})
	return result, apperror.Wrap("open maintenance", err)
}

func (s Service) Start(ctx context.Context, id, actor, request string) (maintenance.Order, error) {
	current, err := s.Store.GetMaintenance(ctx, id)
	if err != nil {
		return maintenance.Order{}, err
	}
	updated, err := current.Start(s.Clock.Now())
	if err != nil {
		return maintenance.Order{}, err
	}
	if err := s.Store.SaveMaintenance(ctx, updated, current.Version); err != nil {
		return maintenance.Order{}, err
	}
	return s.audit(ctx, actor, request, updated, "start")
}

func (s Service) Complete(ctx context.Context, id, actor, request string) (maintenance.Order, error) {
	var result maintenance.Order
	err := s.Store.WithTx(ctx, func(ctx context.Context, tx repository.Tx) error {
		current, err := tx.GetMaintenance(ctx, id)
		if err != nil {
			return err
		}
		vehicleItem, err := tx.GetVehicle(ctx, current.VehicleID)
		if err != nil {
			return err
		}
		now := s.Clock.Now()
		result, err = current.Complete(now)
		if err != nil {
			return err
		}
		if err := tx.SaveMaintenance(ctx, result, current.Version); err != nil {
			return err
		}
		previous := vehicleItem
		vehicleItem, err = vehicleItem.ReleaseMaintenance(now)
		if err != nil {
			return err
		}
		if err := tx.SaveVehicle(ctx, vehicleItem, previous.Version); err != nil {
			return err
		}
		return tx.AppendAudit(ctx, audit.Event{ID: s.IDs.NewID("audit"), ActorID: actor, EntityType: "maintenance", EntityID: id, Action: "complete", Result: "success", RequestID: request, Metadata: map[string]any{}, CreatedAt: now})
	})
	return result, apperror.Wrap("complete maintenance", err)
}

func (s Service) audit(ctx context.Context, actor, request string, order maintenance.Order, action string) (maintenance.Order, error) {
	err := s.Store.AppendAudit(ctx, audit.Event{ID: s.IDs.NewID("audit"), ActorID: actor, EntityType: "maintenance", EntityID: order.ID, Action: action, Result: "success", RequestID: request, Metadata: map[string]any{}, CreatedAt: s.Clock.Now()})
	return order, err
}
