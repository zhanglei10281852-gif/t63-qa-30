package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/ratelimit"
)

func TestRequestIDUsesIncomingValueAndGeneratesMissingValue(t *testing.T) {
	ids := &identity.Sequence{}
	handler := RequestID(ids)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, RequestIDFrom(r.Context()))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "client-request")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Body.String() != "client-request" || recorder.Header().Get("X-Request-ID") != "client-request" {
		t.Fatalf("incoming request id was not propagated")
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if !strings.HasPrefix(recorder.Body.String(), "req_") {
		t.Fatalf("generated id=%q", recorder.Body.String())
	}
}

func TestCORSAllowsConfiguredOriginAndRejectsOthers(t *testing.T) {
	nextCalls := 0
	handler := CORS([]string{"http://localhost:5173"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vehicles", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusAccepted || recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("configured origin response headers=%v status=%d", recorder.Header(), recorder.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/vehicles", nil)
	req.Header.Set("Origin", "https://untrusted.example")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("untrusted origin was reflected")
	}

	req = httptest.NewRequest(http.MethodOptions, "/api/v1/vehicles", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent || nextCalls != 2 {
		t.Fatalf("preflight status=%d nextCalls=%d", recorder.Code, nextCalls)
	}
}

func TestRateLimitReturnsStableJSONAndRetryHeader(t *testing.T) {
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	limiter := ratelimit.New(1, 1)
	limiter.Now = func() time.Time { return now }
	handler := Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), RequestID(&identity.Sequence{}), RateLimit(limiter))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d", first.Code)
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))
	if second.Code != http.StatusTooManyRequests || second.Header().Get("Retry-After") != "1" {
		t.Fatalf("limited response status=%d headers=%v", second.Code, second.Header())
	}
	if !strings.Contains(second.Body.String(), `"code":"rate_limited"`) || !strings.Contains(second.Body.String(), `"request_id":"req_`) {
		t.Fatalf("limited body=%s", second.Body.String())
	}
}

func TestRecoverConvertsPanicToServerError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := Recover(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("panic status=%d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "internal_error") {
		t.Fatalf("panic body=%s", recorder.Body.String())
	}
}
