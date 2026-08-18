package reconciliation

import (
	"context"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/businessday"
	domain "sanitation-operations/internal/domain/reconciliation"
	"sanitation-operations/internal/domain/trip"
	"sanitation-operations/internal/domain/workplan"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
)

type Service struct {
	Store    repository.Store
	Calendar businessday.Calendar
	Now      func() time.Time
}

func (s Service) Evaluate(ctx context.Context, serviceDate string) (domain.Report, error) {
	if serviceDate == "" {
		return domain.Report{}, apperror.Validation(apperror.ErrValidation)
	}
	start, end, err := s.Calendar.Bounds(serviceDate)
	if err != nil {
		return domain.Report{}, apperror.Validation(err)
	}
	shifts, err := s.allShifts(ctx, serviceDate)
	if err != nil {
		return domain.Report{}, apperror.Wrap("load reconciliation shifts", err)
	}
	trips, err := s.allTrips(ctx, start, end)
	if err != nil {
		return domain.Report{}, apperror.Wrap("load reconciliation trips", err)
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	return domain.Evaluate(serviceDate, shifts, trips, now), nil
}

func (s Service) allShifts(ctx context.Context, serviceDate string) ([]workplan.Shift, error) {
	result := []workplan.Shift{}
	for offset := 0; ; offset += 100 {
		page, err := s.Store.ListShifts(ctx, repository.ShiftFilter{ServiceDate: serviceDate}, pagination.Query{Limit: 100, Offset: offset})
		if err != nil {
			return nil, err
		}
		result = append(result, page.Items...)
		if len(result) >= page.Total {
			return result, nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
}
func (s Service) allTrips(ctx context.Context, from, to time.Time) ([]trip.Trip, error) {
	result := []trip.Trip{}
	for offset := 0; ; offset += 100 {
		page, err := s.Store.ListTrips(ctx, repository.TripFilter{From: &from, To: &to}, pagination.Query{Limit: 100, Offset: offset})
		if err != nil {
			return nil, err
		}
		result = append(result, page.Items...)
		if len(result) >= page.Total {
			return result, nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
}
