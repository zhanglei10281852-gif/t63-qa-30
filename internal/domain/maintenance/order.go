package maintenance

import (
	"time"

	"sanitation-operations/internal/apperror"
)

type Status string

const (
	Open       Status = "open"
	InProgress Status = "in_progress"
	Completed  Status = "completed"
	Cancelled  Status = "cancelled"
)

type Order struct {
	ID        string     `json:"id"`
	VehicleID string     `json:"vehicle_id"`
	Kind      string     `json:"kind"`
	Status    Status     `json:"status"`
	OpenedAt  time.Time  `json:"opened_at"`
	DueAt     time.Time  `json:"due_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	Notes     string     `json:"notes"`
	Version   int        `json:"version"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func New(id, vehicleID, kind, notes string, opened, due, now time.Time) (Order, error) {
	if id == "" || vehicleID == "" || kind == "" || !due.After(opened) {
		return Order{}, apperror.Validation(apperror.ErrValidation)
	}
	return Order{ID: id, VehicleID: vehicleID, Kind: kind, Status: Open, OpenedAt: opened.UTC(), DueAt: due.UTC(), Notes: notes, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (o Order) Start(now time.Time) (Order, error) {
	if o.Status != Open {
		return Order{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	o.Status = InProgress
	o.Version++
	o.UpdatedAt = now.UTC()
	return o, nil
}

func (o Order) Complete(now time.Time) (Order, error) {
	if o.Status != InProgress {
		return Order{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	stamp := now.UTC()
	o.Status = Completed
	o.ClosedAt = &stamp
	o.Version++
	o.UpdatedAt = stamp
	return o, nil
}

func (o Order) Cancel(now time.Time) (Order, error) {
	if o.Status == Completed || o.Status == Cancelled {
		return Order{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	o.Status = Cancelled
	o.Version++
	o.UpdatedAt = now.UTC()
	return o, nil
}
