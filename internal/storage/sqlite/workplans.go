package sqlite

import (
	"context"
	"fmt"
	"strings"

	"sanitation-operations/internal/domain/workplan"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
)

const routeColumns = "id, route_code, name, zone, required_capacity_kg, status, created_at, updated_at"
const shiftColumns = "id, route_id, service_date, start_at, end_at, status, assigned_vehicle_id, version, created_at, updated_at"

func (s *queryStore) GetRoute(ctx context.Context, id string) (workplan.Route, error) {
	return scanRoute(s.e.QueryRowContext(ctx, "SELECT "+routeColumns+" FROM routes WHERE id = ?", id))
}

func (s *queryStore) ListRoutes(ctx context.Context, page pagination.Query) (pagination.Result[workplan.Route], error) {
	var total int
	if err := s.e.QueryRowContext(ctx, "SELECT COUNT(*) FROM routes").Scan(&total); err != nil {
		return pagination.Result[workplan.Route]{}, err
	}
	sort := "updated_at"
	if page.Sort == "route_code" {
		sort = "route_code"
	}
	direction := "ASC"
	if page.Desc {
		direction = "DESC"
	}
	rows, err := s.e.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM routes ORDER BY %s %s LIMIT ? OFFSET ?", routeColumns, sort, direction), page.Limit, page.Offset)
	if err != nil {
		return pagination.Result[workplan.Route]{}, err
	}
	defer rows.Close()
	items := make([]workplan.Route, 0, page.Limit)
	for rows.Next() {
		item, err := scanRoute(rows)
		if err != nil {
			return pagination.Result[workplan.Route]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[workplan.Route]{}, err
	}
	return pagination.Result[workplan.Route]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *queryStore) SaveRoute(ctx context.Context, r workplan.Route) error {
	_, err := s.e.ExecContext(ctx, "INSERT INTO routes("+routeColumns+") VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name, zone=excluded.zone, required_capacity_kg=excluded.required_capacity_kg, status=excluded.status, updated_at=excluded.updated_at", r.ID, r.Code, r.Name, r.Zone, r.RequiredCapacityKg, r.Status, formatTime(r.CreatedAt), formatTime(r.UpdatedAt))
	return databaseError(err)
}

func (s *queryStore) GetShift(ctx context.Context, id string) (workplan.Shift, error) {
	return scanShift(s.e.QueryRowContext(ctx, "SELECT "+shiftColumns+" FROM shifts WHERE id = ?", id))
}

func (s *queryStore) ListShifts(ctx context.Context, filter repository.ShiftFilter, page pagination.Query) (pagination.Result[workplan.Shift], error) {
	where := []string{"1 = 1"}
	args := make([]any, 0, 3)
	if filter.ServiceDate != "" {
		where = append(where, "service_date = ?")
		args = append(args, filter.ServiceDate)
	}
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.RouteID != "" {
		where = append(where, "route_id = ?")
		args = append(args, filter.RouteID)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := s.e.QueryRowContext(ctx, "SELECT COUNT(*) FROM shifts WHERE "+clause, args...).Scan(&total); err != nil {
		return pagination.Result[workplan.Shift]{}, err
	}
	direction := "ASC"
	if page.Desc {
		direction = "DESC"
	}
	query := fmt.Sprintf("SELECT %s FROM shifts WHERE %s ORDER BY service_date %s, start_at %s LIMIT ? OFFSET ?", shiftColumns, clause, direction, direction)
	args = append(args, page.Limit, page.Offset)
	rows, err := s.e.QueryContext(ctx, query, args...)
	if err != nil {
		return pagination.Result[workplan.Shift]{}, err
	}
	defer rows.Close()
	items := make([]workplan.Shift, 0, page.Limit)
	for rows.Next() {
		item, err := scanShift(rows)
		if err != nil {
			return pagination.Result[workplan.Shift]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[workplan.Shift]{}, err
	}
	return pagination.Result[workplan.Shift]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *queryStore) SaveShift(ctx context.Context, sh workplan.Shift, expectedVersion int) error {
	var assigned any
	if sh.AssignedVehicleID != nil {
		assigned = *sh.AssignedVehicleID
	}
	if expectedVersion == 0 {
		_, err := s.e.ExecContext(ctx, "INSERT INTO shifts("+shiftColumns+") VALUES(?,?,?,?,?,?,?,?,?,?)", sh.ID, sh.RouteID, sh.ServiceDate, formatTime(sh.StartAt), formatTime(sh.EndAt), sh.Status, assigned, sh.Version, formatTime(sh.CreatedAt), formatTime(sh.UpdatedAt))
		return databaseError(err)
	}
	result, err := s.e.ExecContext(ctx, "UPDATE shifts SET service_date=?, start_at=?, end_at=?, status=?, assigned_vehicle_id=?, version=?, updated_at=? WHERE id=? AND version=?", sh.ServiceDate, formatTime(sh.StartAt), formatTime(sh.EndAt), sh.Status, assigned, sh.Version, formatTime(sh.UpdatedAt), sh.ID, expectedVersion)
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
