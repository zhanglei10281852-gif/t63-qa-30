package planning

import (
	"context"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/audit"
	"sanitation-operations/internal/businessday"
	"sanitation-operations/internal/clock"
	"sanitation-operations/internal/domain/workplan"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
	"sanitation-operations/internal/validation"
)

type Service struct {
	Store    repository.Store
	Clock    clock.Clock
	IDs      identity.Generator
	Calendar *businessday.Calendar
}
type CreateRouteInput struct {
	Code, Name, Zone   string
	RequiredCapacityKg int
	ActorID, RequestID string
}
type CreateShiftInput struct {
	RouteID, ServiceDate string
	StartAt, EndAt       time.Time
	ActorID, RequestID   string
}

func (s Service) CreateRoute(ctx context.Context, input CreateRouteInput) (workplan.Route, error) {
	if err := ctx.Err(); err != nil {
		return workplan.Route{}, apperror.Wrap("create route context", err)
	}
	var checks validation.Collector
	checks.Required("code", input.Code)
	checks.Code("code", input.Code)
	checks.Required("name", input.Name)
	checks.MaxLength("name", input.Name, 120)
	checks.Required("zone", input.Zone)
	checks.Positive("required_capacity_kg", input.RequiredCapacityKg)
	if err := checks.Err(); err != nil {
		return workplan.Route{}, apperror.Validation(err)
	}
	now := s.Clock.Now()
	route, err := workplan.NewRoute(s.IDs.NewID("route"), input.Code, input.Name, input.Zone, input.RequiredCapacityKg, now)
	if err != nil {
		return workplan.Route{}, err
	}
	if err := s.Store.SaveRoute(ctx, route); err != nil {
		return workplan.Route{}, apperror.Wrap("save route", err)
	}
	if err := s.Store.AppendAudit(ctx, audit.Event{ID: s.IDs.NewID("audit"), ActorID: input.ActorID, EntityType: "route", EntityID: route.ID, Action: "create", Result: "success", RequestID: input.RequestID, Metadata: map[string]any{"code": route.Code}, CreatedAt: now}); err != nil {
		return workplan.Route{}, apperror.Wrap("audit route", err)
	}
	return route, nil
}

func (s Service) CreateShift(ctx context.Context, input CreateShiftInput) (workplan.Shift, error) {
	if err := ctx.Err(); err != nil {
		return workplan.Shift{}, apperror.Wrap("create shift context", err)
	}
	var checks validation.Collector
	checks.Required("route_id", input.RouteID)
	checks.Required("service_date", input.ServiceDate)
	checks.Window("start_at", "end_at", input.StartAt, input.EndAt, 20*time.Hour)
	if err := checks.Err(); err != nil {
		return workplan.Shift{}, apperror.Validation(err)
	}
	if s.Calendar != nil {
		if err := s.Calendar.ValidateWindow(input.ServiceDate, input.StartAt, input.EndAt); err != nil {
			return workplan.Shift{}, apperror.Validation(err)
		}
	}
	if _, err := s.Store.GetRoute(ctx, input.RouteID); err != nil {
		return workplan.Shift{}, err
	}
	now := s.Clock.Now()
	shift, err := workplan.NewShift(s.IDs.NewID("shift"), input.RouteID, input.ServiceDate, input.StartAt, input.EndAt, now)
	if err != nil {
		return workplan.Shift{}, err
	}
	if err := s.Store.SaveShift(ctx, shift, 0); err != nil {
		return workplan.Shift{}, apperror.Wrap("save shift", err)
	}
	return shift, nil
}

func (s Service) CancelShift(ctx context.Context, id, actor, request string) (workplan.Shift, error) {
	current, err := s.Store.GetShift(ctx, id)
	if err != nil {
		return workplan.Shift{}, err
	}
	updated, err := current.Cancel(s.Clock.Now())
	if err != nil {
		return workplan.Shift{}, err
	}
	if err := s.Store.SaveShift(ctx, updated, current.Version); err != nil {
		return workplan.Shift{}, apperror.Wrap("cancel shift", err)
	}
	return updated, s.Store.AppendAudit(ctx, audit.Event{ID: s.IDs.NewID("audit"), ActorID: actor, EntityType: "shift", EntityID: id, Action: "cancel", Result: "success", RequestID: request, Metadata: map[string]any{}, CreatedAt: s.Clock.Now()})
}

func (s Service) ListShifts(ctx context.Context, filter repository.ShiftFilter, page pagination.Query) (pagination.Result[workplan.Shift], error) {
	return s.Store.ListShifts(ctx, filter, page)
}
func (s Service) ListRoutes(ctx context.Context, page pagination.Query) (pagination.Result[workplan.Route], error) {
	return s.Store.ListRoutes(ctx, page)
}
