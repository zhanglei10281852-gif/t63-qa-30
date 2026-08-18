package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/clock"
	fueldomain "sanitation-operations/internal/domain/fuel"
	"sanitation-operations/internal/domain/incident"
	inspectiondomain "sanitation-operations/internal/domain/inspection"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/middleware"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
	authservice "sanitation-operations/internal/service/auth"
	"sanitation-operations/internal/service/batch"
	crewservice "sanitation-operations/internal/service/crew"
	"sanitation-operations/internal/service/dispatch"
	"sanitation-operations/internal/service/fleet"
	fuelservice "sanitation-operations/internal/service/fuel"
	inspectionservice "sanitation-operations/internal/service/inspection"
	maintservice "sanitation-operations/internal/service/maintenance"
	"sanitation-operations/internal/service/planning"
	queryservice "sanitation-operations/internal/service/query"
	reconciliationservice "sanitation-operations/internal/service/reconciliation"
)

type Server struct {
	Auth           authservice.Service
	Dispatch       dispatch.Service
	Batch          batch.Service
	Crew           crewservice.Service
	Fleet          fleet.Service
	Fuel           fuelservice.Service
	Inspection     inspectionservice.Service
	Planning       planning.Service
	Maintenance    maintservice.Service
	Query          queryservice.Service
	Reconciliation reconciliationservice.Service
	Ready          func(context.Context) error
	Clock          clock.Clock
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("GET /api/v1/auth/me", s.me)
	mux.HandleFunc("GET /api/v1/vehicles", s.listVehicles)
	mux.HandleFunc("GET /api/v1/drivers", s.listDrivers)
	mux.HandleFunc("POST /api/v1/drivers", s.createDriver)
	mux.HandleFunc("POST /api/v1/drivers/{id}/certifications", s.addDriverCertification)
	mux.HandleFunc("POST /api/v1/drivers/{id}/suspend", s.suspendDriver)
	mux.HandleFunc("POST /api/v1/drivers/{id}/reactivate", s.reactivateDriver)
	mux.HandleFunc("POST /api/v1/vehicles", s.createVehicle)
	mux.HandleFunc("GET /api/v1/shifts", s.listShifts)
	mux.HandleFunc("GET /api/v1/routes", s.listRoutes)
	mux.HandleFunc("GET /api/v1/trips", s.listTrips)
	mux.HandleFunc("GET /api/v1/maintenance", s.listMaintenance)
	mux.HandleFunc("GET /api/v1/incidents", s.listIncidents)
	mux.HandleFunc("GET /api/v1/fuel", s.listFuel)
	mux.HandleFunc("GET /api/v1/inspections", s.listInspections)
	mux.HandleFunc("GET /api/v1/reconciliation", s.reconcile)
	mux.HandleFunc("POST /api/v1/routes", s.createRoute)
	mux.HandleFunc("POST /api/v1/shifts", s.createShift)
	mux.HandleFunc("POST /api/v1/shifts/assign", s.assignShift)
	mux.HandleFunc("POST /api/v1/shifts/batch-assign", s.batchAssign)
	mux.HandleFunc("POST /api/v1/trips/start", s.startTrip)
	mux.HandleFunc("POST /api/v1/incidents", s.reportIncident)
	mux.HandleFunc("POST /api/v1/fuel", s.recordFuel)
	mux.HandleFunc("POST /api/v1/inspections", s.createInspection)
	mux.HandleFunc("POST /api/v1/inspections/{id}/items", s.recordInspectionItem)
	mux.HandleFunc("POST /api/v1/inspections/{id}/submit", s.submitInspection)
	mux.HandleFunc("POST /api/v1/maintenance", s.openMaintenance)
	mux.HandleFunc("POST /api/v1/maintenance/{id}/start", s.startMaintenance)
	mux.HandleFunc("POST /api/v1/maintenance/{id}/complete", s.completeMaintenance)
	mux.HandleFunc("POST /api/v1/trips/{id}/return", s.returnTrip)
	return mux
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s Server) login(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Auth.Login(r.Context(), input.Username, input.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s Server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.Logout(r.Context(), middleware.BearerToken(r)); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s Server) me(w http.ResponseWriter, r *http.Request) {
	value, ok := middleware.OperatorFrom(r.Context())
	if !ok {
		writeError(w, r, apperror.Unauthorized(errors.New("authentication required")))
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s Server) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
func (s Server) ready(w http.ResponseWriter, r *http.Request) {
	if s.Ready != nil {
		if err := s.Ready(r.Context()); err != nil {
			writeError(w, r, apperror.Unavailable(err))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

type routeRequest struct {
	Code, Name, Zone   string
	RequiredCapacityKg int `json:"required_capacity_kg"`
}
type driverRequest struct {
	EmployeeNo       string    `json:"employee_no"`
	Name             string    `json:"name"`
	LicenseClass     string    `json:"license_class"`
	LicenseExpiresAt time.Time `json:"license_expires_at"`
}
type certificationRequest struct {
	Code        string    `json:"code"`
	VehicleType string    `json:"vehicle_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (s Server) createDriver(w http.ResponseWriter, r *http.Request) {
	var input driverRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Crew.Create(r.Context(), crewservice.CreateInput{EmployeeNo: input.EmployeeNo, Name: input.Name, LicenseClass: input.LicenseClass, LicenseExpiresAt: input.LicenseExpiresAt, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
func (s Server) addDriverCertification(w http.ResponseWriter, r *http.Request) {
	var input certificationRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Crew.AddCertification(r.Context(), crewservice.CertificationInput{DriverID: resourceID(r, "drivers"), Code: input.Code, VehicleType: input.VehicleType, ExpiresAt: input.ExpiresAt, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) suspendDriver(w http.ResponseWriter, r *http.Request) {
	result, err := s.Crew.Suspend(r.Context(), resourceID(r, "drivers"), actor(r), middleware.RequestIDFrom(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) reactivateDriver(w http.ResponseWriter, r *http.Request) {
	result, err := s.Crew.Reactivate(r.Context(), resourceID(r, "drivers"), actor(r), middleware.RequestIDFrom(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type vehicleRequest struct {
	PlateNumber     string    `json:"plate_number"`
	VehicleType     string    `json:"vehicle_type"`
	DepotCode       string    `json:"depot_code"`
	CapacityKg      int       `json:"capacity_kg"`
	OdometerKm      int       `json:"odometer_km"`
	InspectionDueAt time.Time `json:"inspection_due_at"`
}

func (s Server) createVehicle(w http.ResponseWriter, r *http.Request) {
	var input vehicleRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Fleet.Create(r.Context(), fleet.CreateInput{PlateNumber: input.PlateNumber, VehicleType: input.VehicleType, DepotCode: input.DepotCode, CapacityKg: input.CapacityKg, OdometerKm: input.OdometerKm, InspectionDueAt: input.InspectionDueAt, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
func (s Server) createRoute(w http.ResponseWriter, r *http.Request) {
	var input routeRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Planning.CreateRoute(r.Context(), planning.CreateRouteInput{Code: input.Code, Name: input.Name, Zone: input.Zone, RequiredCapacityKg: input.RequiredCapacityKg, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

type shiftRequest struct {
	RouteID     string    `json:"route_id"`
	ServiceDate string    `json:"service_date"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`
}

func (s Server) createShift(w http.ResponseWriter, r *http.Request) {
	var input shiftRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Planning.CreateShift(r.Context(), planning.CreateShiftInput{RouteID: input.RouteID, ServiceDate: input.ServiceDate, StartAt: input.StartAt, EndAt: input.EndAt, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

type assignRequest struct {
	ShiftID   string `json:"shift_id"`
	VehicleID string `json:"vehicle_id"`
}

func (s Server) assignShift(w http.ResponseWriter, r *http.Request) {
	var input assignRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Dispatch.AssignShift(r.Context(), dispatch.AssignInput{ShiftID: input.ShiftID, VehicleID: input.VehicleID, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type batchAssignRequest struct {
	Assignments []batch.Assignment `json:"assignments"`
}

func (s Server) batchAssign(w http.ResponseWriter, r *http.Request) {
	var input batchAssignRequest
	if !decode(w, r, &input) {
		return
	}
	results := s.Batch.Assign(r.Context(), input.Assignments, actor(r), middleware.RequestIDFrom(r.Context()))
	writeJSON(w, http.StatusMultiStatus, map[string]any{"results": results})
}

type startRequest struct {
	VehicleID      string `json:"vehicle_id"`
	ShiftID        string `json:"shift_id"`
	DriverID       string `json:"driver_id"`
	DriverName     string `json:"driver_name"`
	IdempotencyKey string `json:"idempotency_key"`
	StartOdometer  int    `json:"start_odometer"`
	LoadKg         int    `json:"load_kg"`
}

func (s Server) startTrip(w http.ResponseWriter, r *http.Request) {
	var input startRequest
	if !decode(w, r, &input) {
		return
	}
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = r.Header.Get("Idempotency-Key")
	}
	result, err := s.Dispatch.StartTrip(r.Context(), dispatch.StartTripInput{VehicleID: input.VehicleID, ShiftID: input.ShiftID, DriverID: input.DriverID, DriverName: input.DriverName, IdempotencyKey: input.IdempotencyKey, StartOdometer: input.StartOdometer, LoadKg: input.LoadKg, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

type returnRequest struct {
	EndOdometer int `json:"end_odometer"`
}

func (s Server) returnTrip(w http.ResponseWriter, r *http.Request) {
	var input returnRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Dispatch.ReturnTrip(r.Context(), dispatch.ReturnTripInput{TripID: resourceID(r, "trips"), EndOdometer: input.EndOdometer, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type incidentRequest struct {
	TripID     string            `json:"trip_id"`
	Severity   incident.Severity `json:"severity"`
	Summary    string            `json:"summary"`
	OccurredAt time.Time         `json:"occurred_at"`
}

type fuelRequest struct {
	VehicleID   string          `json:"vehicle_id"`
	FuelType    fueldomain.Type `json:"fuel_type"`
	Quantity    float64         `json:"quantity"`
	CostCents   int64           `json:"cost_cents"`
	OdometerKm  int             `json:"odometer_km"`
	StationCode string          `json:"station_code"`
	RecordedAt  time.Time       `json:"recorded_at"`
}

func (s Server) recordFuel(w http.ResponseWriter, r *http.Request) {
	var input fuelRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Fuel.Record(r.Context(), fuelservice.RecordInput{VehicleID: input.VehicleID, FuelType: input.FuelType, Quantity: input.Quantity, CostCents: input.CostCents, OdometerKm: input.OdometerKm, StationCode: input.StationCode, RecordedAt: input.RecordedAt, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s Server) reportIncident(w http.ResponseWriter, r *http.Request) {
	var input incidentRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Dispatch.ReportIncident(r.Context(), dispatch.IncidentInput{TripID: input.TripID, Severity: input.Severity, Summary: input.Summary, OccurredAt: input.OccurredAt, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

type inspectionRequest struct {
	VehicleID   string    `json:"vehicle_id"`
	Inspector   string    `json:"inspector"`
	InspectedAt time.Time `json:"inspected_at"`
}
type inspectionItemRequest struct {
	Code   string                  `json:"code"`
	Result inspectiondomain.Result `json:"result"`
	Notes  string                  `json:"notes"`
}

func (s Server) createInspection(w http.ResponseWriter, r *http.Request) {
	var input inspectionRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Inspection.Create(r.Context(), inspectionservice.CreateInput{VehicleID: input.VehicleID, Inspector: input.Inspector, InspectedAt: input.InspectedAt, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
func (s Server) recordInspectionItem(w http.ResponseWriter, r *http.Request) {
	var input inspectionItemRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Inspection.Record(r.Context(), inspectionservice.RecordInput{InspectionID: resourceID(r, "inspections"), Code: input.Code, Result: input.Result, Notes: input.Notes, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) submitInspection(w http.ResponseWriter, r *http.Request) {
	result, err := s.Inspection.Submit(r.Context(), resourceID(r, "inspections"), actor(r), middleware.RequestIDFrom(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type maintenanceRequest struct {
	VehicleID string    `json:"vehicle_id"`
	Kind      string    `json:"kind"`
	Notes     string    `json:"notes"`
	DueAt     time.Time `json:"due_at"`
}

func (s Server) openMaintenance(w http.ResponseWriter, r *http.Request) {
	var input maintenanceRequest
	if !decode(w, r, &input) {
		return
	}
	result, err := s.Maintenance.Open(r.Context(), maintservice.OpenInput{VehicleID: input.VehicleID, Kind: input.Kind, Notes: input.Notes, DueAt: input.DueAt, ActorID: actor(r), RequestID: middleware.RequestIDFrom(r.Context())})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
func (s Server) startMaintenance(w http.ResponseWriter, r *http.Request) {
	result, err := s.Maintenance.Start(r.Context(), resourceID(r, "maintenance"), actor(r), middleware.RequestIDFrom(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) completeMaintenance(w http.ResponseWriter, r *http.Request) {
	result, err := s.Maintenance.Complete(r.Context(), resourceID(r, "maintenance"), actor(r), middleware.RequestIDFrom(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s Server) listVehicles(w http.ResponseWriter, r *http.Request) {
	page, err := page(r)
	if err != nil {
		writeError(w, r, apperror.Validation(err))
		return
	}
	result, err := s.Query.Vehicles(r.Context(), repository.VehicleFilter{Status: vehicleStatus(r.URL.Query().Get("status")), Depot: r.URL.Query().Get("depot"), Query: r.URL.Query().Get("q")}, page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) listDrivers(w http.ResponseWriter, r *http.Request) {
	page, err := page(r)
	if err != nil {
		writeError(w, r, apperror.Validation(err))
		return
	}
	result, err := s.Crew.List(r.Context(), r.URL.Query().Get("status"), page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) listShifts(w http.ResponseWriter, r *http.Request) {
	page, err := page(r)
	if err != nil {
		writeError(w, r, apperror.Validation(err))
		return
	}
	result, err := s.Query.Shifts(r.Context(), repository.ShiftFilter{ServiceDate: r.URL.Query().Get("service_date"), RouteID: r.URL.Query().Get("route_id")}, page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) listRoutes(w http.ResponseWriter, r *http.Request) {
	page, err := page(r)
	if err != nil {
		writeError(w, r, apperror.Validation(err))
		return
	}
	result, err := s.Query.Store.ListRoutes(r.Context(), page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) listTrips(w http.ResponseWriter, r *http.Request) {
	page, err := page(r)
	if err != nil {
		writeError(w, r, apperror.Validation(err))
		return
	}
	result, err := s.Query.Trips(r.Context(), repository.TripFilter{VehicleID: r.URL.Query().Get("vehicle_id")}, page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) listMaintenance(w http.ResponseWriter, r *http.Request) {
	page, err := page(r)
	if err != nil {
		writeError(w, r, apperror.Validation(err))
		return
	}
	result, err := s.Query.Maintenance(r.Context(), r.URL.Query().Get("vehicle_id"), page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) listIncidents(w http.ResponseWriter, r *http.Request) {
	page, err := page(r)
	if err != nil {
		writeError(w, r, apperror.Validation(err))
		return
	}
	result, err := s.Query.Incidents(r.Context(), page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) listFuel(w http.ResponseWriter, r *http.Request) {
	page, err := page(r)
	if err != nil {
		writeError(w, r, apperror.Validation(err))
		return
	}
	result, err := s.Fuel.List(r.Context(), r.URL.Query().Get("vehicle_id"), page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) listInspections(w http.ResponseWriter, r *http.Request) {
	page, err := page(r)
	if err != nil {
		writeError(w, r, apperror.Validation(err))
		return
	}
	result, err := s.Query.Inspections(r.Context(), r.URL.Query().Get("vehicle_id"), page)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) reconcile(w http.ResponseWriter, r *http.Request) {
	result, err := s.Reconciliation.Evaluate(r.Context(), r.URL.Query().Get("service_date"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func page(r *http.Request) (pagination.Query, error) {
	query := r.URL.Query()
	desc := query.Get("order") == "desc"
	return pagination.Parse(query.Get("limit"), query.Get("offset"), query.Get("sort"), desc)
}
func actor(r *http.Request) string {
	if value, ok := middleware.OperatorFrom(r.Context()); ok {
		return value.ID
	}
	value := r.Header.Get("X-Operator-ID")
	if value == "" {
		return "operator:anonymous"
	}
	return value
}
func resourceID(r *http.Request, resource string) string {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for index := range parts {
		if parts[index] == resource && index+1 < len(parts) {
			return parts[index+1]
		}
	}
	return ""
}
func vehicleStatus(value string) vehicle.Status { return vehicle.Status(value) }
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, r, apperror.Validation(fmt.Errorf("invalid JSON: %w", err)))
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := apperror.CodeInternal
	message := "internal server error"
	var app *apperror.AppError
	if errors.As(err, &app) {
		status, code = app.Status, app.Code
		message = app.Err.Error()
	}
	writeJSON(w, status, map[string]any{"code": code, "message": message, "request_id": middleware.RequestIDFrom(r.Context())})
}
