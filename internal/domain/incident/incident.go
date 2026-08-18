package incident

import (
	"time"

	"sanitation-operations/internal/apperror"
)

type Severity string

const (
	Low      Severity = "low"
	Medium   Severity = "medium"
	High     Severity = "high"
	Critical Severity = "critical"
)

type Status string

const (
	Reported      Status = "reported"
	Investigating Status = "investigating"
	Resolved      Status = "resolved"
)

type Incident struct {
	ID         string     `json:"id"`
	TripID     string     `json:"trip_id"`
	VehicleID  string     `json:"vehicle_id"`
	Severity   Severity   `json:"severity"`
	Status     Status     `json:"status"`
	OccurredAt time.Time  `json:"occurred_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	Summary    string     `json:"summary"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func New(id, tripID, vehicleID string, severity Severity, summary string, occurred, now time.Time) (Incident, error) {
	if id == "" || tripID == "" || vehicleID == "" || summary == "" || occurred.After(now) {
		return Incident{}, apperror.Validation(apperror.ErrValidation)
	}
	return Incident{ID: id, TripID: tripID, VehicleID: vehicleID, Severity: severity, Status: Reported, OccurredAt: occurred.UTC(), Summary: summary, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (i Incident) Investigate(now time.Time) (Incident, error) {
	if i.Status != Reported {
		return Incident{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	i.Status = Investigating
	i.UpdatedAt = now.UTC()
	return i, nil
}

func (i Incident) Resolve(now time.Time) (Incident, error) {
	if i.Status != Investigating {
		return Incident{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	stamp := now.UTC()
	i.Status = Resolved
	i.ResolvedAt = &stamp
	i.UpdatedAt = stamp
	return i, nil
}
