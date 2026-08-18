package sqlite

import (
	"context"
	"fmt"
	"strings"

	"sanitation-operations/internal/domain/maintenance"
	"sanitation-operations/internal/pagination"
)

const maintenanceColumns = "id, vehicle_id, kind, status, opened_at, due_at, closed_at, notes, version, created_at, updated_at"

func (s *queryStore) GetMaintenance(ctx context.Context, id string) (maintenance.Order, error) {
	return scanMaintenance(s.e.QueryRowContext(ctx, "SELECT "+maintenanceColumns+" FROM maintenance_orders WHERE id = ?", id))
}

func (s *queryStore) ActiveMaintenanceForVehicle(ctx context.Context, vehicleID string) (maintenance.Order, bool, error) {
	item, err := scanMaintenance(s.e.QueryRowContext(ctx, "SELECT "+maintenanceColumns+" FROM maintenance_orders WHERE vehicle_id = ? AND status IN ('open','in_progress') ORDER BY opened_at DESC LIMIT 1", vehicleID))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return maintenance.Order{}, false, nil
		}
		return maintenance.Order{}, false, err
	}
	return item, true, nil
}

func (s *queryStore) ListMaintenance(ctx context.Context, vehicleID string, page pagination.Query) (pagination.Result[maintenance.Order], error) {
	where := "1 = 1"
	args := []any{}
	if vehicleID != "" {
		where = "vehicle_id = ?"
		args = append(args, vehicleID)
	}
	var total int
	if err := s.e.QueryRowContext(ctx, "SELECT COUNT(*) FROM maintenance_orders WHERE "+where, args...).Scan(&total); err != nil {
		return pagination.Result[maintenance.Order]{}, err
	}
	args = append(args, page.Limit, page.Offset)
	rows, err := s.e.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM maintenance_orders WHERE %s ORDER BY due_at ASC LIMIT ? OFFSET ?", maintenanceColumns, where), args...)
	if err != nil {
		return pagination.Result[maintenance.Order]{}, err
	}
	defer rows.Close()
	items := make([]maintenance.Order, 0, page.Limit)
	for rows.Next() {
		item, err := scanMaintenance(rows)
		if err != nil {
			return pagination.Result[maintenance.Order]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[maintenance.Order]{}, err
	}
	return pagination.Result[maintenance.Order]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *queryStore) SaveMaintenance(ctx context.Context, o maintenance.Order, expectedVersion int) error {
	var closed any
	if o.ClosedAt != nil {
		closed = formatTime(*o.ClosedAt)
	}
	if expectedVersion == 0 {
		_, err := s.e.ExecContext(ctx, "INSERT INTO maintenance_orders("+maintenanceColumns+") VALUES(?,?,?,?,?,?,?,?,?,?,?)", o.ID, o.VehicleID, o.Kind, o.Status, formatTime(o.OpenedAt), formatTime(o.DueAt), closed, o.Notes, o.Version, formatTime(o.CreatedAt), formatTime(o.UpdatedAt))
		return databaseError(err)
	}
	result, err := s.e.ExecContext(ctx, "UPDATE maintenance_orders SET kind=?, status=?, due_at=?, closed_at=?, notes=?, version=?, updated_at=? WHERE id=? AND version=?", o.Kind, o.Status, formatTime(o.DueAt), closed, o.Notes, o.Version, formatTime(o.UpdatedAt), o.ID, expectedVersion)
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
