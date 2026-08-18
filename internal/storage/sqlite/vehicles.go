package sqlite

import (
	"context"
	"fmt"
	"strings"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
)

const vehicleColumns = "id, plate_number, vehicle_type, depot_code, status, capacity_kg, odometer_km, inspection_due_at, version, created_at, updated_at"

func (s *queryStore) GetVehicle(ctx context.Context, id string) (vehicle.Vehicle, error) {
	row := s.e.QueryRowContext(ctx, "SELECT "+vehicleColumns+" FROM vehicles WHERE id = ?", id)
	return scanVehicle(row)
}

func (s *queryStore) ListVehicles(ctx context.Context, filter repository.VehicleFilter, page pagination.Query) (pagination.Result[vehicle.Vehicle], error) {
	where := []string{"1 = 1"}
	args := make([]any, 0, 3)
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Depot != "" {
		where = append(where, "depot_code = ?")
		args = append(args, filter.Depot)
	}
	if filter.Query != "" {
		where = append(where, "(plate_number LIKE ? OR vehicle_type LIKE ?)")
		value := "%" + filter.Query + "%"
		args = append(args, value, value)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := s.e.QueryRowContext(ctx, "SELECT COUNT(*) FROM vehicles WHERE "+clause, args...).Scan(&total); err != nil {
		return pagination.Result[vehicle.Vehicle]{}, err
	}
	sort := "updated_at"
	if page.Sort == "plate_number" {
		sort = "plate_number"
	}
	if page.Sort == "status" {
		sort = "status"
	}
	direction := "ASC"
	if page.Desc {
		direction = "DESC"
	}
	query := fmt.Sprintf("SELECT %s FROM vehicles WHERE %s ORDER BY %s %s LIMIT ? OFFSET ?", vehicleColumns, clause, sort, direction)
	args = append(args, page.Limit, page.Offset)
	rows, err := s.e.QueryContext(ctx, query, args...)
	if err != nil {
		return pagination.Result[vehicle.Vehicle]{}, err
	}
	defer rows.Close()
	items := make([]vehicle.Vehicle, 0, page.Limit)
	for rows.Next() {
		item, err := scanVehicle(rows)
		if err != nil {
			return pagination.Result[vehicle.Vehicle]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[vehicle.Vehicle]{}, err
	}
	return pagination.Result[vehicle.Vehicle]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *queryStore) SaveVehicle(ctx context.Context, v vehicle.Vehicle, expectedVersion int) error {
	if expectedVersion == 0 {
		_, err := s.e.ExecContext(ctx, "INSERT INTO vehicles("+vehicleColumns+") VALUES(?,?,?,?,?,?,?,?,?,?,?)", v.ID, v.PlateNumber, v.VehicleType, v.DepotCode, v.Status, v.CapacityKg, v.OdometerKm, formatTime(v.InspectionDueAt), v.Version, formatTime(v.CreatedAt), formatTime(v.UpdatedAt))
		return databaseError(err)
	}
	result, err := s.e.ExecContext(ctx, "UPDATE vehicles SET plate_number=?, vehicle_type=?, depot_code=?, status=?, capacity_kg=?, odometer_km=?, inspection_due_at=?, version=?, updated_at=? WHERE id=? AND version=?", v.PlateNumber, v.VehicleType, v.DepotCode, v.Status, v.CapacityKg, v.OdometerKm, formatTime(v.InspectionDueAt), v.Version, formatTime(v.UpdatedAt), v.ID, expectedVersion)
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

func repositoryConflict() error { return apperror.Conflict(fmt.Errorf("optimistic version conflict")) }
