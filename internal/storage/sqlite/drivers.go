package sqlite

import (
	"context"
	"fmt"
	"strings"

	"sanitation-operations/internal/domain/crew"
	"sanitation-operations/internal/pagination"
)

const driverColumns = "id, employee_no, name, status, license_class, license_expires_at, version, created_at, updated_at"

func scanDriver(s scanner) (crew.Driver, error) {
	var value crew.Driver
	var expires, created, updated string
	if err := s.Scan(&value.ID, &value.EmployeeNo, &value.Name, &value.Status, &value.LicenseClass, &expires, &value.Version, &created, &updated); err != nil {
		return crew.Driver{}, notFound(err)
	}
	value.LicenseExpiresAt, value.CreatedAt, value.UpdatedAt = parseTime(expires), parseTime(created), parseTime(updated)
	value.Certifications = []crew.Certification{}
	return value, nil
}

func (s *queryStore) GetDriver(ctx context.Context, id string) (crew.Driver, error) {
	value, err := scanDriver(s.e.QueryRowContext(ctx, "SELECT "+driverColumns+" FROM drivers WHERE id=?", id))
	if err != nil {
		return crew.Driver{}, err
	}
	if err := s.loadCertifications(ctx, &value); err != nil {
		return crew.Driver{}, err
	}
	return value, nil
}

func (s *queryStore) ListDrivers(ctx context.Context, status string, page pagination.Query) (pagination.Result[crew.Driver], error) {
	where := "1=1"
	args := []any{}
	if status != "" {
		where = "status=?"
		args = append(args, status)
	}
	var total int
	if err := s.e.QueryRowContext(ctx, "SELECT COUNT(*) FROM drivers WHERE "+where, args...).Scan(&total); err != nil {
		return pagination.Result[crew.Driver]{}, err
	}
	args = append(args, page.Limit, page.Offset)
	direction := "ASC"
	if page.Desc {
		direction = "DESC"
	}
	rows, err := s.e.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM drivers WHERE %s ORDER BY employee_no %s LIMIT ? OFFSET ?", driverColumns, where, direction), args...)
	if err != nil {
		return pagination.Result[crew.Driver]{}, err
	}
	defer rows.Close()
	items := make([]crew.Driver, 0, page.Limit)
	for rows.Next() {
		value, err := scanDriver(rows)
		if err != nil {
			return pagination.Result[crew.Driver]{}, err
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[crew.Driver]{}, err
	}
	for index := range items {
		if err := s.loadCertifications(ctx, &items[index]); err != nil {
			return pagination.Result[crew.Driver]{}, err
		}
	}
	return pagination.Result[crew.Driver]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *queryStore) loadCertifications(ctx context.Context, target *crew.Driver) error {
	rows, err := s.e.QueryContext(ctx, "SELECT id, driver_id, certification_code, vehicle_type, expires_at FROM driver_certifications WHERE driver_id=? ORDER BY certification_code", target.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var value crew.Certification
		var expires string
		if err := rows.Scan(&value.ID, &value.DriverID, &value.Code, &value.VehicleType, &expires); err != nil {
			return err
		}
		value.ExpiresAt = parseTime(expires)
		target.Certifications = append(target.Certifications, value)
	}
	return rows.Err()
}

func (s *queryStore) SaveDriver(ctx context.Context, value crew.Driver, expectedVersion int) error {
	if expectedVersion == 0 {
		if _, err := s.e.ExecContext(ctx, "INSERT INTO drivers("+driverColumns+") VALUES(?,?,?,?,?,?,?,?,?)", value.ID, value.EmployeeNo, value.Name, value.Status, value.LicenseClass, formatTime(value.LicenseExpiresAt), value.Version, formatTime(value.CreatedAt), formatTime(value.UpdatedAt)); err != nil {
			return databaseError(err)
		}
	} else {
		result, err := s.e.ExecContext(ctx, "UPDATE drivers SET employee_no=?, name=?, status=?, license_class=?, license_expires_at=?, version=?, updated_at=? WHERE id=? AND version=?", value.EmployeeNo, value.Name, value.Status, value.LicenseClass, formatTime(value.LicenseExpiresAt), value.Version, formatTime(value.UpdatedAt), value.ID, expectedVersion)
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
		if _, err := s.e.ExecContext(ctx, "DELETE FROM driver_certifications WHERE driver_id=?", value.ID); err != nil {
			return err
		}
	}
	for _, cert := range value.Certifications {
		if _, err := s.e.ExecContext(ctx, "INSERT INTO driver_certifications(id, driver_id, certification_code, vehicle_type, expires_at) VALUES(?,?,?,?,?)", cert.ID, value.ID, cert.Code, cert.VehicleType, formatTime(cert.ExpiresAt)); err != nil {
			return databaseError(err)
		}
	}
	return nil
}

func hasNoDriver(err error) bool { return strings.Contains(strings.ToLower(err.Error()), "not found") }
