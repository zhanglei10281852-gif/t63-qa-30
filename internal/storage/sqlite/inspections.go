package sqlite

import (
	"context"
	"fmt"
	"strings"

	"sanitation-operations/internal/domain/inspection"
	"sanitation-operations/internal/pagination"
)

const inspectionColumns = "id, vehicle_id, inspector, status, inspected_at, expires_at, score, version, created_at, updated_at"

func scanInspectionHeader(s scanner) (inspection.Inspection, error) {
	var value inspection.Inspection
	var inspected, expires, created, updated string
	if err := s.Scan(&value.ID, &value.VehicleID, &value.Inspector, &value.Status, &inspected, &expires, &value.Score, &value.Version, &created, &updated); err != nil {
		return inspection.Inspection{}, notFound(err)
	}
	value.InspectedAt, value.ExpiresAt, value.CreatedAt, value.UpdatedAt = parseTime(inspected), parseTime(expires), parseTime(created), parseTime(updated)
	value.Items = []inspection.Item{}
	return value, nil
}

func (s *queryStore) GetInspection(ctx context.Context, id string) (inspection.Inspection, error) {
	value, err := scanInspectionHeader(s.e.QueryRowContext(ctx, "SELECT "+inspectionColumns+" FROM inspections WHERE id=?", id))
	if err != nil {
		return inspection.Inspection{}, err
	}
	if err := s.loadInspectionItems(ctx, &value); err != nil {
		return inspection.Inspection{}, err
	}
	return value, nil
}

func (s *queryStore) LatestInspectionForVehicle(ctx context.Context, vehicleID string) (inspection.Inspection, bool, error) {
	value, err := scanInspectionHeader(s.e.QueryRowContext(ctx, "SELECT "+inspectionColumns+" FROM inspections WHERE vehicle_id=? ORDER BY inspected_at DESC LIMIT 1", vehicleID))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return inspection.Inspection{}, false, nil
		}
		return inspection.Inspection{}, false, err
	}
	if err := s.loadInspectionItems(ctx, &value); err != nil {
		return inspection.Inspection{}, false, err
	}
	return value, true, nil
}

func (s *queryStore) ListInspections(ctx context.Context, vehicleID string, page pagination.Query) (pagination.Result[inspection.Inspection], error) {
	where := "1=1"
	args := []any{}
	if vehicleID != "" {
		where = "vehicle_id=?"
		args = append(args, vehicleID)
	}
	var total int
	if err := s.e.QueryRowContext(ctx, "SELECT COUNT(*) FROM inspections WHERE "+where, args...).Scan(&total); err != nil {
		return pagination.Result[inspection.Inspection]{}, err
	}
	args = append(args, page.Limit, page.Offset)
	rows, err := s.e.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM inspections WHERE %s ORDER BY inspected_at DESC LIMIT ? OFFSET ?", inspectionColumns, where), args...)
	if err != nil {
		return pagination.Result[inspection.Inspection]{}, err
	}
	defer rows.Close()
	headers := make([]inspection.Inspection, 0, page.Limit)
	for rows.Next() {
		value, err := scanInspectionHeader(rows)
		if err != nil {
			return pagination.Result[inspection.Inspection]{}, err
		}
		headers = append(headers, value)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[inspection.Inspection]{}, err
	}
	for index := range headers {
		if err := s.loadInspectionItems(ctx, &headers[index]); err != nil {
			return pagination.Result[inspection.Inspection]{}, err
		}
	}
	return pagination.Result[inspection.Inspection]{Items: headers, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *queryStore) loadInspectionItems(ctx context.Context, target *inspection.Inspection) error {
	rows, err := s.e.QueryContext(ctx, "SELECT id, inspection_id, item_code, result, notes FROM inspection_items WHERE inspection_id=? ORDER BY item_code", target.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item inspection.Item
		if err := rows.Scan(&item.ID, &item.InspectionID, &item.Code, &item.Result, &item.Notes); err != nil {
			return err
		}
		target.Items = append(target.Items, item)
	}
	return rows.Err()
}

func (s *queryStore) SaveInspection(ctx context.Context, value inspection.Inspection, expectedVersion int) error {
	if expectedVersion == 0 {
		if _, err := s.e.ExecContext(ctx, "INSERT INTO inspections("+inspectionColumns+") VALUES(?,?,?,?,?,?,?,?,?,?)", value.ID, value.VehicleID, value.Inspector, value.Status, formatTime(value.InspectedAt), formatTime(value.ExpiresAt), value.Score, value.Version, formatTime(value.CreatedAt), formatTime(value.UpdatedAt)); err != nil {
			return databaseError(err)
		}
	} else {
		result, err := s.e.ExecContext(ctx, "UPDATE inspections SET inspector=?, status=?, inspected_at=?, expires_at=?, score=?, version=?, updated_at=? WHERE id=? AND version=?", value.Inspector, value.Status, formatTime(value.InspectedAt), formatTime(value.ExpiresAt), value.Score, value.Version, formatTime(value.UpdatedAt), value.ID, expectedVersion)
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
		if _, err := s.e.ExecContext(ctx, "DELETE FROM inspection_items WHERE inspection_id=?", value.ID); err != nil {
			return err
		}
	}
	for _, item := range value.Items {
		if _, err := s.e.ExecContext(ctx, "INSERT INTO inspection_items(id, inspection_id, item_code, result, notes) VALUES(?,?,?,?,?)", item.ID, value.ID, item.Code, item.Result, item.Notes); err != nil {
			return databaseError(err)
		}
	}
	return nil
}
