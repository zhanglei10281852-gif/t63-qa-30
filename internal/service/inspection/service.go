package inspection

import (
	"context"
	"encoding/json"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/audit"
	"sanitation-operations/internal/clock"
	domain "sanitation-operations/internal/domain/inspection"
	"sanitation-operations/internal/domain/maintenance"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/repository"
)

type Service struct {
	Store    repository.Store
	Clock    clock.Clock
	IDs      identity.Generator
	Validity time.Duration
}
type CreateInput struct {
	VehicleID, Inspector, ActorID, RequestID string
	InspectedAt                              time.Time
}
type RecordInput struct {
	InspectionID, Code        string
	Result                    domain.Result
	Notes, ActorID, RequestID string
}

func (s Service) Create(ctx context.Context, input CreateInput) (domain.Inspection, error) {
	var result domain.Inspection
	err := s.Store.WithTx(ctx, func(ctx context.Context, tx repository.Tx) error {
		vehicleItem, err := tx.GetVehicle(ctx, input.VehicleID)
		if err != nil {
			return err
		}
		if vehicleItem.Status == vehicle.OnDuty || vehicleItem.Status == vehicle.Retired {
			return apperror.Conflict(apperror.ErrUnavailable)
		}
		now := s.Clock.Now()
		inspected := input.InspectedAt
		if inspected.IsZero() {
			inspected = now
		}
		result, err = domain.New(s.IDs.NewID("inspection"), vehicleItem.ID, input.Inspector, inspected, inspected.Add(s.validity()), now)
		if err != nil {
			return err
		}
		if err := tx.SaveInspection(ctx, result, 0); err != nil {
			return err
		}
		return tx.AppendAudit(ctx, s.event(input.ActorID, input.RequestID, result.ID, "create", map[string]any{"vehicle_id": vehicleItem.ID}))
	})
	return result, apperror.Wrap("create inspection", err)
}

func (s Service) Record(ctx context.Context, input RecordInput) (domain.Inspection, error) {
	current, err := s.Store.GetInspection(ctx, input.InspectionID)
	if err != nil {
		return domain.Inspection{}, err
	}
	item := domain.Item{ID: s.IDs.NewID("check"), InspectionID: current.ID, Code: input.Code, Result: input.Result, Notes: input.Notes}
	updated, err := current.Record(item, s.Clock.Now())
	if err != nil {
		return domain.Inspection{}, err
	}
	if err := s.Store.SaveInspection(ctx, updated, current.Version); err != nil {
		return domain.Inspection{}, apperror.Wrap("record inspection item", err)
	}
	return updated, nil
}

func (s Service) Submit(ctx context.Context, id, actor, request string) (domain.Inspection, error) {
	var result domain.Inspection
	err := s.Store.WithTx(ctx, func(ctx context.Context, tx repository.Tx) error {
		current, err := tx.GetInspection(ctx, id)
		if err != nil {
			return err
		}
		result, err = current.Submit(s.Clock.Now())
		if err != nil {
			return err
		}
		if err := tx.SaveInspection(ctx, result, current.Version); err != nil {
			return err
		}
		if result.Status == domain.Failed {
			vehicleItem, err := tx.GetVehicle(ctx, result.VehicleID)
			if err != nil {
				return err
			}
			previous := vehicleItem
			vehicleItem, err = vehicleItem.MarkMaintenance(s.Clock.Now())
			if err != nil {
				return err
			}
			if err := tx.SaveVehicle(ctx, vehicleItem, previous.Version); err != nil {
				return err
			}
			order, err := maintenance.New(s.IDs.NewID("maint"), vehicleItem.ID, "inspection_failure", "Opened automatically from failed compliance inspection", s.Clock.Now(), s.Clock.Now().Add(7*24*time.Hour), s.Clock.Now())
			if err != nil {
				return err
			}
			if err := tx.SaveMaintenance(ctx, order, 0); err != nil {
				return err
			}
		}
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err := tx.Enqueue(ctx, "inspection.submitted", data, s.Clock.Now()); err != nil {
			return err
		}
		return tx.AppendAudit(ctx, s.event(actor, request, result.ID, "submit", map[string]any{"status": result.Status, "score": result.Score}))
	})
	return result, apperror.Wrap("submit inspection", err)
}

func (s Service) event(actor, request, id, action string, metadata map[string]any) audit.Event {
	return audit.Event{ID: s.IDs.NewID("audit"), ActorID: actor, EntityType: "inspection", EntityID: id, Action: action, Result: "success", RequestID: request, Metadata: metadata, CreatedAt: s.Clock.Now()}
}
func (s Service) validity() time.Duration {
	if s.Validity <= 0 {
		return 30 * 24 * time.Hour
	}
	return s.Validity
}
