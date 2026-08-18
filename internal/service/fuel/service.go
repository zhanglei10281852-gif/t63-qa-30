package fuel

import (
	"context"
	"encoding/json"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/audit"
	"sanitation-operations/internal/clock"
	domain "sanitation-operations/internal/domain/fuel"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
)

type Service struct {
	Store repository.Store
	Clock clock.Clock
	IDs   identity.Generator
}
type RecordInput struct {
	VehicleID                       string
	FuelType                        domain.Type
	Quantity                        float64
	CostCents                       int64
	OdometerKm                      int
	StationCode, ActorID, RequestID string
	RecordedAt                      time.Time
}
type RecordResult struct {
	Record        domain.Record `json:"record"`
	Efficiency    float64       `json:"efficiency,omitempty"`
	HasEfficiency bool          `json:"has_efficiency"`
}

func (s Service) Record(ctx context.Context, input RecordInput) (RecordResult, error) {
	var result RecordResult
	err := s.Store.WithTx(ctx, func(ctx context.Context, tx repository.Tx) error {
		vehicleItem, err := tx.GetVehicle(ctx, input.VehicleID)
		if err != nil {
			return err
		}
		if vehicleItem.Status == vehicle.OnDuty || vehicleItem.Status == vehicle.Retired || input.OdometerKm < vehicleItem.OdometerKm {
			return apperror.Conflict(apperror.ErrInvalidState)
		}
		now := s.Clock.Now()
		recorded := input.RecordedAt
		if recorded.IsZero() {
			recorded = now
		}
		result.Record, err = domain.New(s.IDs.NewID("fuel"), vehicleItem.ID, input.FuelType, input.Quantity, input.CostCents, input.OdometerKm, input.StationCode, recorded, now)
		if err != nil {
			return err
		}
		previousFuel, exists, err := tx.LatestFuel(ctx, vehicleItem.ID)
		if err != nil {
			return err
		}
		if exists {
			result.Efficiency, result.HasEfficiency = domain.Efficiency(previousFuel, result.Record)
		}
		if err := tx.SaveFuel(ctx, result.Record); err != nil {
			return err
		}
		if input.OdometerKm > vehicleItem.OdometerKm {
			previousVersion := vehicleItem.Version
			vehicleItem.OdometerKm = input.OdometerKm
			vehicleItem.Version++
			vehicleItem.UpdatedAt = now
			if err := tx.SaveVehicle(ctx, vehicleItem, previousVersion); err != nil {
				return err
			}
		}
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err := tx.Enqueue(ctx, "fuel.recorded", data, now); err != nil {
			return err
		}
		return tx.AppendAudit(ctx, audit.Event{ID: s.IDs.NewID("audit"), ActorID: input.ActorID, EntityType: "fuel", EntityID: result.Record.ID, Action: "record", Result: "success", RequestID: input.RequestID, Metadata: map[string]any{"vehicle_id": vehicleItem.ID, "quantity": input.Quantity, "unit": result.Record.Unit}, CreatedAt: now})
	})
	return result, apperror.Wrap("record fuel", err)
}

func (s Service) List(ctx context.Context, vehicleID string, page pagination.Query) (pagination.Result[domain.Record], error) {
	return s.Store.ListFuel(ctx, vehicleID, page)
}
