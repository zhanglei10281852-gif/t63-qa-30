package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"sanitation-operations/internal/businessday"
	"sanitation-operations/internal/clock"
	"sanitation-operations/internal/domain/inspection"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/middleware"
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
	"sanitation-operations/internal/storage/sqlite"
)

type apiHarness struct {
	handler http.Handler
	now     time.Time
}

func newAPIHarness(t *testing.T) apiHarness {
	t.Helper()
	ctx := context.Background()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "api.db"))
	store, err := sqlite.Open(ctx, "file:"+path+"?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	clk := clock.Fixed{Current: now}
	ids := &identity.Sequence{}
	calendar, err := businessday.New("Asia/Shanghai", 4)
	if err != nil {
		t.Fatal(err)
	}
	dispatchService := dispatch.Service{Store: store, Clock: clk, IDs: ids}
	server := Server{
		Dispatch:       dispatchService,
		Batch:          batch.Service{Dispatch: dispatchService, MaxParallel: 2},
		Crew:           crewservice.Service{Store: store, Clock: clk, IDs: ids},
		Fleet:          fleet.Service{Store: store, Clock: clk, IDs: ids},
		Fuel:           fuelservice.Service{Store: store, Clock: clk, IDs: ids},
		Inspection:     inspectionservice.Service{Store: store, Clock: clk, IDs: ids},
		Planning:       planning.Service{Store: store, Clock: clk, IDs: ids, Calendar: &calendar},
		Maintenance:    maintservice.Service{Store: store, Clock: clk, IDs: ids},
		Query:          queryservice.Service{Store: store},
		Reconciliation: reconciliationservice.Service{Store: store, Calendar: calendar, Now: clk.Now},
		Clock:          clk,
		Ready:          store.Ping,
	}
	handler := middleware.Chain(server.Handler(), middleware.RequestID(ids))
	return apiHarness{handler: handler, now: now}
}

func (h apiHarness) request(t *testing.T, method, path string, body any, wantStatus int) map[string]any {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Operator-ID", "operator:test")
	req.Header.Set("X-Request-ID", "request-test")
	recorder := httptest.NewRecorder()
	h.handler.ServeHTTP(recorder, req)
	if recorder.Code != wantStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, recorder.Code, wantStatus, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	return response
}

func requireString(t *testing.T, value map[string]any, key string) string {
	t.Helper()
	text, ok := value[key].(string)
	if !ok || text == "" {
		t.Fatalf("response field %q is missing: %+v", key, value)
	}
	return text
}

func (h apiHarness) createVehicle(t *testing.T, suffix string) map[string]any {
	t.Helper()
	return h.request(t, http.MethodPost, "/api/v1/vehicles", map[string]any{
		"plate_number":      "沪A00" + suffix,
		"vehicle_type":      "compactor",
		"depot_code":        "H-01",
		"capacity_kg":       9000,
		"odometer_km":       100,
		"inspection_due_at": h.now.AddDate(1, 0, 0),
	}, http.StatusCreated)
}

func (h apiHarness) createDriver(t *testing.T, suffix string) map[string]any {
	t.Helper()
	driver := h.request(t, http.MethodPost, "/api/v1/drivers", map[string]any{
		"employee_no":        "DRV-" + suffix,
		"name":               "测试驾驶员" + suffix,
		"license_class":      "B2",
		"license_expires_at": h.now.AddDate(1, 0, 0),
	}, http.StatusCreated)
	id := requireString(t, driver, "id")
	h.request(t, http.MethodPost, "/api/v1/drivers/"+id+"/certifications", map[string]any{
		"code":         "CERT-" + suffix,
		"vehicle_type": "compactor",
		"expires_at":   h.now.AddDate(1, 0, 0),
	}, http.StatusOK)
	return driver
}

func (h apiHarness) createRouteAndShift(t *testing.T, suffix string) (map[string]any, map[string]any) {
	t.Helper()
	route := h.request(t, http.MethodPost, "/api/v1/routes", map[string]any{
		"code":                 "H-" + suffix,
		"name":                 "测试清运路线" + suffix,
		"zone":                 "north",
		"required_capacity_kg": 5000,
	}, http.StatusCreated)
	shift := h.request(t, http.MethodPost, "/api/v1/shifts", map[string]any{
		"route_id":     requireString(t, route, "id"),
		"service_date": "2026-08-18",
		"start_at":     h.now.Add(time.Hour),
		"end_at":       h.now.Add(3 * time.Hour),
	}, http.StatusCreated)
	return route, shift
}

func (h apiHarness) passInspection(t *testing.T, vehicleID string) map[string]any {
	t.Helper()
	value := h.request(t, http.MethodPost, "/api/v1/inspections", map[string]any{
		"vehicle_id":   vehicleID,
		"inspector":    "quality:test",
		"inspected_at": h.now,
	}, http.StatusCreated)
	id := requireString(t, value, "id")
	for _, code := range inspection.RequiredItems {
		h.request(t, http.MethodPost, "/api/v1/inspections/"+id+"/items", map[string]any{
			"code":   code,
			"result": "pass",
			"notes":  "checked",
		}, http.StatusOK)
	}
	return h.request(t, http.MethodPost, "/api/v1/inspections/"+id+"/submit", nil, http.StatusOK)
}

func TestHealthAndErrorContract(t *testing.T) {
	h := newAPIHarness(t)
	if status := requireString(t, h.request(t, http.MethodGet, "/health/live", nil, http.StatusOK), "status"); status != "ok" {
		t.Fatalf("live status=%s", status)
	}
	if status := requireString(t, h.request(t, http.MethodGet, "/health/ready", nil, http.StatusOK), "status"); status != "ready" {
		t.Fatalf("ready status=%s", status)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vehicles", bytes.NewBufferString(`{"unknown":true}`))
	req.Header.Set("X-Request-ID", "request-invalid")
	recorder := httptest.NewRecorder()
	h.handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "validation_error" || body["request_id"] != "request-invalid" {
		t.Fatalf("unexpected error body %+v", body)
	}
}

func TestVehicleDriverFilteringAndLifecycle(t *testing.T) {
	h := newAPIHarness(t)
	vehicle := h.createVehicle(t, "201")
	driver := h.createDriver(t, "201")

	vehicles := h.request(t, http.MethodGet, "/api/v1/vehicles?limit=10&status=available&q=201", nil, http.StatusOK)
	if vehicles["total"].(float64) != 1 {
		t.Fatalf("vehicle filter response %+v", vehicles)
	}
	drivers := h.request(t, http.MethodGet, "/api/v1/drivers?limit=10&status=active", nil, http.StatusOK)
	if drivers["total"].(float64) != 1 {
		t.Fatalf("driver filter response %+v", drivers)
	}

	driverID := requireString(t, driver, "id")
	suspended := h.request(t, http.MethodPost, "/api/v1/drivers/"+driverID+"/suspend", nil, http.StatusOK)
	if suspended["status"] != "suspended" {
		t.Fatalf("driver was not suspended: %+v", suspended)
	}
	reactivated := h.request(t, http.MethodPost, "/api/v1/drivers/"+driverID+"/reactivate", nil, http.StatusOK)
	if reactivated["status"] != "active" {
		t.Fatalf("driver was not reactivated: %+v", reactivated)
	}
	if requireString(t, vehicle, "status") != "available" {
		t.Fatalf("new vehicle status %+v", vehicle)
	}
}

func TestCompleteDispatchFlowIsTransactionalAndIdempotent(t *testing.T) {
	h := newAPIHarness(t)
	vehicle := h.createVehicle(t, "301")
	driver := h.createDriver(t, "301")
	_, shift := h.createRouteAndShift(t, "301")
	vehicleID := requireString(t, vehicle, "id")
	shiftID := requireString(t, shift, "id")
	driverID := requireString(t, driver, "id")
	h.passInspection(t, vehicleID)

	assigned := h.request(t, http.MethodPost, "/api/v1/shifts/assign", map[string]any{
		"shift_id":   shiftID,
		"vehicle_id": vehicleID,
	}, http.StatusOK)
	if assigned["status"] != "assigned" {
		t.Fatalf("shift not assigned: %+v", assigned)
	}

	startBody := map[string]any{
		"vehicle_id":      vehicleID,
		"shift_id":        shiftID,
		"driver_id":       driverID,
		"driver_name":     driver["name"],
		"idempotency_key": "dispatch-301",
		"start_odometer":  100,
		"load_kg":         6000,
	}
	started := h.request(t, http.MethodPost, "/api/v1/trips/start", startBody, http.StatusCreated)
	replayed := h.request(t, http.MethodPost, "/api/v1/trips/start", startBody, http.StatusCreated)
	if started["id"] != replayed["id"] {
		t.Fatalf("idempotent replay changed trip: first=%+v replay=%+v", started, replayed)
	}
	tripID := requireString(t, started, "id")
	returned := h.request(t, http.MethodPost, "/api/v1/trips/"+tripID+"/return", map[string]any{"end_odometer": 145}, http.StatusOK)
	if returned["status"] != "completed" {
		t.Fatalf("trip not completed: %+v", returned)
	}

	vehiclePage := h.request(t, http.MethodGet, "/api/v1/vehicles?limit=10&q=301", nil, http.StatusOK)
	items := vehiclePage["items"].([]any)
	current := items[0].(map[string]any)
	if current["status"] != "available" || current["odometer_km"].(float64) != 145 {
		t.Fatalf("vehicle not restored after return: %+v", current)
	}
}

func TestFailedInspectionCreatesMaintenanceBlock(t *testing.T) {
	h := newAPIHarness(t)
	vehicle := h.createVehicle(t, "401")
	vehicleID := requireString(t, vehicle, "id")
	created := h.request(t, http.MethodPost, "/api/v1/inspections", map[string]any{
		"vehicle_id":   vehicleID,
		"inspector":    "quality:test",
		"inspected_at": h.now,
	}, http.StatusCreated)
	id := requireString(t, created, "id")
	for index, code := range inspection.RequiredItems {
		result := "pass"
		if index == 0 {
			result = "fail"
		}
		h.request(t, http.MethodPost, "/api/v1/inspections/"+id+"/items", map[string]any{
			"code": code, "result": result, "notes": fmt.Sprintf("item %d", index),
		}, http.StatusOK)
	}
	submitted := h.request(t, http.MethodPost, "/api/v1/inspections/"+id+"/submit", nil, http.StatusOK)
	if submitted["status"] != "failed" {
		t.Fatalf("inspection should fail: %+v", submitted)
	}
	maintenance := h.request(t, http.MethodGet, "/api/v1/maintenance?limit=10&vehicle_id="+vehicleID, nil, http.StatusOK)
	if maintenance["total"].(float64) != 1 {
		t.Fatalf("automatic maintenance missing: %+v", maintenance)
	}
	vehicles := h.request(t, http.MethodGet, "/api/v1/vehicles?limit=10&q=401", nil, http.StatusOK)
	current := vehicles["items"].([]any)[0].(map[string]any)
	if current["status"] != "maintenance" {
		t.Fatalf("vehicle not blocked: %+v", current)
	}
}

func TestFuelAndManualMaintenanceContracts(t *testing.T) {
	h := newAPIHarness(t)
	createdVehicle := h.createVehicle(t, "501")
	vehicleID := requireString(t, createdVehicle, "id")

	fuelResult := h.request(t, http.MethodPost, "/api/v1/fuel", map[string]any{
		"vehicle_id": vehicleID, "fuel_type": "diesel", "quantity": 32.5,
		"cost_cents": 26000, "odometer_km": 120, "station_code": "H-01", "recorded_at": h.now,
	}, http.StatusCreated)
	record, ok := fuelResult["record"].(map[string]any)
	if !ok || record["vehicle_id"] != vehicleID || fuelResult["has_efficiency"] != false {
		t.Fatalf("unexpected fuel response: %+v", fuelResult)
	}
	fuelPage := h.request(t, http.MethodGet, "/api/v1/fuel?limit=100", nil, http.StatusOK)
	if fuelPage["total"].(float64) != 1 {
		t.Fatalf("fuel record was not listed: %+v", fuelPage)
	}

	opened := h.request(t, http.MethodPost, "/api/v1/maintenance", map[string]any{
		"vehicle_id": vehicleID, "kind": "scheduled", "notes": "annual service", "due_at": h.now.Add(48 * time.Hour),
	}, http.StatusCreated)
	maintenanceID := requireString(t, opened, "id")
	started := h.request(t, http.MethodPost, "/api/v1/maintenance/"+maintenanceID+"/start", nil, http.StatusOK)
	if started["status"] != "in_progress" {
		t.Fatalf("maintenance did not start: %+v", started)
	}
	completed := h.request(t, http.MethodPost, "/api/v1/maintenance/"+maintenanceID+"/complete", nil, http.StatusOK)
	if completed["status"] != "completed" {
		t.Fatalf("maintenance did not complete: %+v", completed)
	}
}

func TestPaginationValidationAndReconciliation(t *testing.T) {
	h := newAPIHarness(t)
	errorBody := h.request(t, http.MethodGet, "/api/v1/vehicles?limit=101", nil, http.StatusBadRequest)
	if errorBody["code"] != "validation_error" {
		t.Fatalf("unexpected pagination error %+v", errorBody)
	}
	report := h.request(t, http.MethodGet, "/api/v1/reconciliation?service_date=2026-08-18", nil, http.StatusOK)
	if report["service_date"] != "2026-08-18" {
		t.Fatalf("unexpected reconciliation %+v", report)
	}
	h.request(t, http.MethodGet, "/api/v1/reconciliation", nil, http.StatusBadRequest)
}
