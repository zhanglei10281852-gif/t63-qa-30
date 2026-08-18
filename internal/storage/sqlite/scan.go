package sqlite

import (
	"database/sql"
	"time"

	"sanitation-operations/internal/domain/incident"
	"sanitation-operations/internal/domain/maintenance"
	"sanitation-operations/internal/domain/trip"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/domain/workplan"
)

type scanner interface{ Scan(...any) error }

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func scanVehicle(s scanner) (vehicle.Vehicle, error) {
	var v vehicle.Vehicle
	var due, created, updated string
	err := s.Scan(&v.ID, &v.PlateNumber, &v.VehicleType, &v.DepotCode, &v.Status, &v.CapacityKg, &v.OdometerKm, &due, &v.Version, &created, &updated)
	if err != nil {
		return vehicle.Vehicle{}, notFound(err)
	}
	v.InspectionDueAt, v.CreatedAt, v.UpdatedAt = parseTime(due), parseTime(created), parseTime(updated)
	return v, nil
}

func scanRoute(s scanner) (workplan.Route, error) {
	var r workplan.Route
	var created, updated string
	err := s.Scan(&r.ID, &r.Code, &r.Name, &r.Zone, &r.RequiredCapacityKg, &r.Status, &created, &updated)
	if err != nil {
		return workplan.Route{}, notFound(err)
	}
	r.CreatedAt, r.UpdatedAt = parseTime(created), parseTime(updated)
	return r, nil
}

func scanShift(s scanner) (workplan.Shift, error) {
	var sh workplan.Shift
	var start, end, created, updated string
	var assigned sql.NullString
	err := s.Scan(&sh.ID, &sh.RouteID, &sh.ServiceDate, &start, &end, &sh.Status, &assigned, &sh.Version, &created, &updated)
	if err != nil {
		return workplan.Shift{}, notFound(err)
	}
	sh.StartAt, sh.EndAt, sh.CreatedAt, sh.UpdatedAt = parseTime(start), parseTime(end), parseTime(created), parseTime(updated)
	if assigned.Valid {
		sh.AssignedVehicleID = &assigned.String
	}
	return sh, nil
}

func scanTrip(s scanner) (trip.Trip, error) {
	var t trip.Trip
	var started, ended, created, updated sql.NullString
	var startOdo, endOdo sql.NullInt64
	err := s.Scan(&t.ID, &t.VehicleID, &t.ShiftID, &t.DriverID, &t.Status, &t.DriverName, &started, &ended, &startOdo, &endOdo, &t.LoadKg, &t.IdempotencyKey, &t.Version, &created, &updated)
	if err != nil {
		return trip.Trip{}, notFound(err)
	}
	if started.Valid {
		value := parseTime(started.String)
		t.StartedAt = &value
	}
	if ended.Valid {
		value := parseTime(ended.String)
		t.EndedAt = &value
	}
	if startOdo.Valid {
		value := int(startOdo.Int64)
		t.StartOdometer = &value
	}
	if endOdo.Valid {
		value := int(endOdo.Int64)
		t.EndOdometer = &value
	}
	t.CreatedAt, t.UpdatedAt = parseTime(created.String), parseTime(updated.String)
	return t, nil
}

func scanMaintenance(s scanner) (maintenance.Order, error) {
	var o maintenance.Order
	var opened, due, closed, created, updated string
	var closedValue sql.NullString
	err := s.Scan(&o.ID, &o.VehicleID, &o.Kind, &o.Status, &opened, &due, &closedValue, &o.Notes, &o.Version, &created, &updated)
	if err != nil {
		return maintenance.Order{}, notFound(err)
	}
	o.OpenedAt, o.DueAt, o.CreatedAt, o.UpdatedAt = parseTime(opened), parseTime(due), parseTime(created), parseTime(updated)
	if closedValue.Valid {
		closed = closedValue.String
		value := parseTime(closed)
		o.ClosedAt = &value
	}
	return o, nil
}

func scanIncident(s scanner) (incident.Incident, error) {
	var i incident.Incident
	var occurred, resolved, created, updated string
	var resolvedValue sql.NullString
	err := s.Scan(&i.ID, &i.TripID, &i.VehicleID, &i.Severity, &i.Status, &occurred, &resolvedValue, &i.Summary, &created, &updated)
	if err != nil {
		return incident.Incident{}, notFound(err)
	}
	i.OccurredAt, i.CreatedAt, i.UpdatedAt = parseTime(occurred), parseTime(created), parseTime(updated)
	if resolvedValue.Valid {
		resolved = resolvedValue.String
		value := parseTime(resolved)
		i.ResolvedAt = &value
	}
	return i, nil
}
