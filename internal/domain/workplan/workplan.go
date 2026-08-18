package workplan

import (
	"time"

	"sanitation-operations/internal/apperror"
)

type RouteStatus string

const (
	RouteActive RouteStatus = "active"
	RoutePaused RouteStatus = "paused"
	RouteClosed RouteStatus = "closed"
)

type Route struct {
	ID                 string      `json:"id"`
	Code               string      `json:"route_code"`
	Name               string      `json:"name"`
	Zone               string      `json:"zone"`
	RequiredCapacityKg int         `json:"required_capacity_kg"`
	Status             RouteStatus `json:"status"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
}

func NewRoute(id, code, name, zone string, required int, now time.Time) (Route, error) {
	if id == "" || code == "" || name == "" || zone == "" || required <= 0 {
		return Route{}, apperror.Validation(apperror.ErrValidation)
	}
	return Route{ID: id, Code: code, Name: name, Zone: zone, RequiredCapacityKg: required, Status: RouteActive, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

type ShiftStatus string

const (
	Draft      ShiftStatus = "draft"
	Scheduled  ShiftStatus = "scheduled"
	Assigned   ShiftStatus = "assigned"
	InProgress ShiftStatus = "in_progress"
	Completed  ShiftStatus = "completed"
	Cancelled  ShiftStatus = "cancelled"
)

type Shift struct {
	ID                string      `json:"id"`
	RouteID           string      `json:"route_id"`
	ServiceDate       string      `json:"service_date"`
	StartAt           time.Time   `json:"start_at"`
	EndAt             time.Time   `json:"end_at"`
	Status            ShiftStatus `json:"status"`
	AssignedVehicleID *string     `json:"assigned_vehicle_id,omitempty"`
	Version           int         `json:"version"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

func NewShift(id, routeID, serviceDate string, start, end, now time.Time) (Shift, error) {
	if id == "" || routeID == "" || serviceDate == "" || !end.After(start) {
		return Shift{}, apperror.Validation(apperror.ErrValidation)
	}
	return Shift{ID: id, RouteID: routeID, ServiceDate: serviceDate, StartAt: start.UTC(), EndAt: end.UTC(), Status: Scheduled, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (s Shift) Assign(vehicleID string, now time.Time) (Shift, error) {
	if s.Status != Scheduled || vehicleID == "" {
		return Shift{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	s.AssignedVehicleID = &vehicleID
	s.Status = Assigned
	s.Version++
	s.UpdatedAt = now.UTC()
	return s, nil
}

func (s Shift) Start(now time.Time) (Shift, error) {
	if s.Status != Assigned || s.AssignedVehicleID == nil {
		return Shift{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	s.Status = InProgress
	s.Version++
	s.UpdatedAt = now.UTC()
	return s, nil
}

func (s Shift) Complete(now time.Time) (Shift, error) {
	if s.Status != InProgress {
		return Shift{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	s.Status = Completed
	s.Version++
	s.UpdatedAt = now.UTC()
	return s, nil
}

func (s Shift) Cancel(now time.Time) (Shift, error) {
	if s.Status == Completed || s.Status == Cancelled || s.Status == InProgress {
		return Shift{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	s.Status = Cancelled
	s.Version++
	s.UpdatedAt = now.UTC()
	return s, nil
}
