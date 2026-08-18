package crew

import (
	"context"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/audit"
	"sanitation-operations/internal/clock"
	domain "sanitation-operations/internal/domain/crew"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
	"sanitation-operations/internal/validation"
)

type Service struct {
	Store repository.Store
	Clock clock.Clock
	IDs   identity.Generator
}
type CreateInput struct {
	EmployeeNo, Name, LicenseClass, ActorID, RequestID string
	LicenseExpiresAt                                   time.Time
}
type CertificationInput struct {
	DriverID, Code, VehicleType, ActorID, RequestID string
	ExpiresAt                                       time.Time
}

func (s Service) Create(ctx context.Context, input CreateInput) (domain.Driver, error) {
	var checks validation.Collector
	checks.Required("employee_no", input.EmployeeNo)
	checks.Code("employee_no", input.EmployeeNo)
	checks.Required("name", input.Name)
	checks.Required("license_class", input.LicenseClass)
	checks.Future("license_expires_at", input.LicenseExpiresAt, s.Clock.Now())
	if err := checks.Err(); err != nil {
		return domain.Driver{}, apperror.Validation(err)
	}
	now := s.Clock.Now()
	value, err := domain.New(s.IDs.NewID("driver"), input.EmployeeNo, input.Name, input.LicenseClass, input.LicenseExpiresAt, now)
	if err != nil {
		return domain.Driver{}, err
	}
	if err := s.Store.SaveDriver(ctx, value, 0); err != nil {
		return domain.Driver{}, apperror.Wrap("save driver", err)
	}
	return value, s.Store.AppendAudit(ctx, s.event(input.ActorID, input.RequestID, value.ID, "create", map[string]any{"employee_no": value.EmployeeNo}))
}

func (s Service) AddCertification(ctx context.Context, input CertificationInput) (domain.Driver, error) {
	current, err := s.Store.GetDriver(ctx, input.DriverID)
	if err != nil {
		return domain.Driver{}, err
	}
	value := domain.Certification{ID: s.IDs.NewID("cert"), DriverID: current.ID, Code: input.Code, VehicleType: input.VehicleType, ExpiresAt: input.ExpiresAt}
	updated, err := current.AddCertification(value, s.Clock.Now())
	if err != nil {
		return domain.Driver{}, err
	}
	if err := s.Store.SaveDriver(ctx, updated, current.Version); err != nil {
		return domain.Driver{}, apperror.Wrap("save certification", err)
	}
	return updated, s.Store.AppendAudit(ctx, s.event(input.ActorID, input.RequestID, updated.ID, "certify", map[string]any{"vehicle_type": input.VehicleType}))
}

func (s Service) Suspend(ctx context.Context, id, actor, request string) (domain.Driver, error) {
	current, err := s.Store.GetDriver(ctx, id)
	if err != nil {
		return domain.Driver{}, err
	}
	updated, err := current.Suspend(s.Clock.Now())
	if err != nil {
		return domain.Driver{}, err
	}
	if err := s.Store.SaveDriver(ctx, updated, current.Version); err != nil {
		return domain.Driver{}, err
	}
	return updated, s.Store.AppendAudit(ctx, s.event(actor, request, id, "suspend", map[string]any{}))
}
func (s Service) Reactivate(ctx context.Context, id, actor, request string) (domain.Driver, error) {
	current, err := s.Store.GetDriver(ctx, id)
	if err != nil {
		return domain.Driver{}, err
	}
	updated, err := current.Reactivate(s.Clock.Now())
	if err != nil {
		return domain.Driver{}, err
	}
	if err := s.Store.SaveDriver(ctx, updated, current.Version); err != nil {
		return domain.Driver{}, err
	}
	return updated, s.Store.AppendAudit(ctx, s.event(actor, request, id, "reactivate", map[string]any{}))
}
func (s Service) List(ctx context.Context, status string, page pagination.Query) (pagination.Result[domain.Driver], error) {
	return s.Store.ListDrivers(ctx, status, page)
}
func (s Service) event(actor, request, id, action string, metadata map[string]any) audit.Event {
	return audit.Event{ID: s.IDs.NewID("audit"), ActorID: actor, EntityType: "driver", EntityID: id, Action: action, Result: "success", RequestID: request, Metadata: metadata, CreatedAt: s.Clock.Now()}
}
