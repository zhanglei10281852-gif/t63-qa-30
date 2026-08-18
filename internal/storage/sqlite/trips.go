package sqlite

import (
	"context"
	"fmt"
	"strings"

	"sanitation-operations/internal/domain/trip"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
)

const tripColumns = "id, vehicle_id, shift_id, driver_id, status, driver_name, started_at, ended_at, start_odo, end_odo, load_kg, idempotency_key, version, created_at, updated_at"

func (s *queryStore) GetTrip(ctx context.Context, id string) (trip.Trip, error) {
	return scanTrip(s.e.QueryRowContext(ctx, "SELECT "+tripColumns+" FROM trips WHERE id = ?", id))
}

func (s *queryStore) FindTripByKey(ctx context.Context, vehicleID, key string) (trip.Trip, bool, error) {
	item, err := scanTrip(s.e.QueryRowContext(ctx, "SELECT "+tripColumns+" FROM trips WHERE vehicle_id = ? AND idempotency_key = ?", vehicleID, key))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return trip.Trip{}, false, nil
		}
		return trip.Trip{}, false, err
	}
	return item, true, nil
}

func (s *queryStore) ListTrips(ctx context.Context, filter repository.TripFilter, page pagination.Query) (pagination.Result[trip.Trip], error) {
	where := []string{"1 = 1"}
	args := make([]any, 0, 5)
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.VehicleID != "" {
		where = append(where, "vehicle_id = ?")
		args = append(args, filter.VehicleID)
	}
	if filter.From != nil {
		where = append(where, "started_at >= ?")
		args = append(args, formatTime(*filter.From))
	}
	if filter.To != nil {
		where = append(where, "started_at < ?")
		args = append(args, formatTime(*filter.To))
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := s.e.QueryRowContext(ctx, "SELECT COUNT(*) FROM trips WHERE "+clause, args...).Scan(&total); err != nil {
		return pagination.Result[trip.Trip]{}, err
	}
	direction := "ASC"
	if page.Desc {
		direction = "DESC"
	}
	args = append(args, page.Limit, page.Offset)
	rows, err := s.e.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM trips WHERE %s ORDER BY created_at %s LIMIT ? OFFSET ?", tripColumns, clause, direction), args...)
	if err != nil {
		return pagination.Result[trip.Trip]{}, err
	}
	defer rows.Close()
	items := make([]trip.Trip, 0, page.Limit)
	for rows.Next() {
		item, err := scanTrip(rows)
		if err != nil {
			return pagination.Result[trip.Trip]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[trip.Trip]{}, err
	}
	return pagination.Result[trip.Trip]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *queryStore) SaveTrip(ctx context.Context, t trip.Trip, expectedVersion int) error {
	var started, ended, startOdo, endOdo any
	if t.StartedAt != nil {
		started = formatTime(*t.StartedAt)
	}
	if t.EndedAt != nil {
		ended = formatTime(*t.EndedAt)
	}
	if t.StartOdometer != nil {
		startOdo = *t.StartOdometer
	}
	if t.EndOdometer != nil {
		endOdo = *t.EndOdometer
	}
	if expectedVersion == 0 {
		_, err := s.e.ExecContext(ctx, "INSERT INTO trips("+tripColumns+") VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", t.ID, t.VehicleID, t.ShiftID, t.DriverID, t.Status, t.DriverName, started, ended, startOdo, endOdo, t.LoadKg, t.IdempotencyKey, t.Version, formatTime(t.CreatedAt), formatTime(t.UpdatedAt))
		return databaseError(err)
	}
	result, err := s.e.ExecContext(ctx, "UPDATE trips SET status=?, driver_name=?, started_at=?, ended_at=?, start_odo=?, end_odo=?, load_kg=?, version=?, updated_at=? WHERE id=? AND version=?", t.Status, t.DriverName, started, ended, startOdo, endOdo, t.LoadKg, t.Version, formatTime(t.UpdatedAt), t.ID, expectedVersion)
	if err != nil {
		return databaseError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return repositoryConflict()
	}
	return nil
}
