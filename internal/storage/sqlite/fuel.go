package sqlite

import (
	"context"
	"fmt"
	"strings"

	"sanitation-operations/internal/domain/fuel"
	"sanitation-operations/internal/pagination"
)

const fuelColumns = "id, vehicle_id, fuel_type, quantity, unit, cost_cents, odometer_km, station_code, recorded_at, created_at"

func scanFuel(s scanner) (fuel.Record, error) {
	var value fuel.Record
	var recorded, created string
	if err := s.Scan(&value.ID, &value.VehicleID, &value.FuelType, &value.Quantity, &value.Unit, &value.CostCents, &value.OdometerKm, &value.StationCode, &recorded, &created); err != nil {
		return fuel.Record{}, notFound(err)
	}
	value.RecordedAt, value.CreatedAt = parseTime(recorded), parseTime(created)
	return value, nil
}

func (s *queryStore) LatestFuel(ctx context.Context, vehicleID string) (fuel.Record, bool, error) {
	value, err := scanFuel(s.e.QueryRowContext(ctx, "SELECT "+fuelColumns+" FROM fuel_logs WHERE vehicle_id=? ORDER BY recorded_at DESC LIMIT 1", vehicleID))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return fuel.Record{}, false, nil
		}
		return fuel.Record{}, false, err
	}
	return value, true, nil
}

func (s *queryStore) ListFuel(ctx context.Context, vehicleID string, page pagination.Query) (pagination.Result[fuel.Record], error) {
	where := "1=1"
	args := []any{}
	if vehicleID != "" {
		where = "vehicle_id=?"
		args = append(args, vehicleID)
	}
	var total int
	if err := s.e.QueryRowContext(ctx, "SELECT COUNT(*) FROM fuel_logs WHERE "+where, args...).Scan(&total); err != nil {
		return pagination.Result[fuel.Record]{}, err
	}
	args = append(args, page.Limit, page.Offset)
	rows, err := s.e.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM fuel_logs WHERE %s ORDER BY recorded_at DESC LIMIT ? OFFSET ?", fuelColumns, where), args...)
	if err != nil {
		return pagination.Result[fuel.Record]{}, err
	}
	defer rows.Close()
	items := make([]fuel.Record, 0, page.Limit)
	for rows.Next() {
		value, err := scanFuel(rows)
		if err != nil {
			return pagination.Result[fuel.Record]{}, err
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[fuel.Record]{}, err
	}
	return pagination.Result[fuel.Record]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *queryStore) SaveFuel(ctx context.Context, value fuel.Record) error {
	_, err := s.e.ExecContext(ctx, "INSERT INTO fuel_logs("+fuelColumns+") VALUES(?,?,?,?,?,?,?,?,?,?)", value.ID, value.VehicleID, value.FuelType, value.Quantity, value.Unit, value.CostCents, value.OdometerKm, value.StationCode, formatTime(value.RecordedAt), formatTime(value.CreatedAt))
	return databaseError(err)
}
