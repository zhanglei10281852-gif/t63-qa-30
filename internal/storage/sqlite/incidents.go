package sqlite

import (
	"context"
	"fmt"

	"sanitation-operations/internal/domain/incident"
	"sanitation-operations/internal/pagination"
)

const incidentColumns = "id, trip_id, vehicle_id, severity, status, occurred_at, resolved_at, summary, created_at, updated_at"

func (s *queryStore) GetIncident(ctx context.Context, id string) (incident.Incident, error) {
	return scanIncident(s.e.QueryRowContext(ctx, "SELECT "+incidentColumns+" FROM incidents WHERE id = ?", id))
}

func (s *queryStore) ListIncidents(ctx context.Context, page pagination.Query) (pagination.Result[incident.Incident], error) {
	var total int
	if err := s.e.QueryRowContext(ctx, "SELECT COUNT(*) FROM incidents").Scan(&total); err != nil {
		return pagination.Result[incident.Incident]{}, err
	}
	direction := "DESC"
	if !page.Desc {
		direction = "ASC"
	}
	rows, err := s.e.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM incidents ORDER BY occurred_at %s LIMIT ? OFFSET ?", incidentColumns, direction), page.Limit, page.Offset)
	if err != nil {
		return pagination.Result[incident.Incident]{}, err
	}
	defer rows.Close()
	items := make([]incident.Incident, 0, page.Limit)
	for rows.Next() {
		item, err := scanIncident(rows)
		if err != nil {
			return pagination.Result[incident.Incident]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[incident.Incident]{}, err
	}
	return pagination.Result[incident.Incident]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *queryStore) SaveIncident(ctx context.Context, i incident.Incident) error {
	var resolved any
	if i.ResolvedAt != nil {
		resolved = formatTime(*i.ResolvedAt)
	}
	_, err := s.e.ExecContext(ctx, "INSERT INTO incidents("+incidentColumns+") VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET severity=excluded.severity, status=excluded.status, resolved_at=excluded.resolved_at, summary=excluded.summary, updated_at=excluded.updated_at", i.ID, i.TripID, i.VehicleID, i.Severity, i.Status, formatTime(i.OccurredAt), resolved, i.Summary, formatTime(i.CreatedAt), formatTime(i.UpdatedAt))
	return databaseError(err)
}
