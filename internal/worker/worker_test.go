package worker

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"sanitation-operations/internal/clock"
	"sanitation-operations/internal/repository"
	"sanitation-operations/internal/storage/sqlite"
)

func workerStore(t *testing.T) (*sqlite.DB, context.Context, time.Time) {
	t.Helper()
	ctx := context.Background()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "worker.db"))
	store, err := sqlite.Open(ctx, "file:"+path+"?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, ctx, time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
}

func TestRunnerMarksSuccessfulJobsDone(t *testing.T) {
	store, ctx, now := workerStore(t)
	id, err := store.Enqueue(ctx, "trip.started", []byte(`{"trip_id":"t1"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	var handled repository.OutboxJob
	runner := Runner{
		Store:       store,
		Clock:       clock.Fixed{Current: now},
		MaxAttempts: 3,
		BatchSize:   10,
		Handler: HandlerFunc(func(_ context.Context, job repository.OutboxJob) error {
			handled = job
			return nil
		}),
	}
	if err := runner.process(ctx, 3, 10); err != nil {
		t.Fatal(err)
	}
	if handled.ID != id || handled.Type != "trip.started" {
		t.Fatalf("unexpected handled job %+v", handled)
	}
	jobs, err := store.ClaimDue(ctx, now.Add(time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("done job was reclaimed: %+v", jobs)
	}
}

func TestRunnerSchedulesRetryWithExponentialDelay(t *testing.T) {
	store, ctx, now := workerStore(t)
	if _, err := store.Enqueue(ctx, "fuel.recorded", []byte(`{"fuel_id":"f1"}`), now); err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		Store: store,
		Clock: clock.Fixed{Current: now},
		Handler: HandlerFunc(func(context.Context, repository.OutboxJob) error {
			return errors.New("temporary downstream failure")
		}),
	}
	if err := runner.process(ctx, 3, 10); err != nil {
		t.Fatal(err)
	}
	jobs, err := store.ClaimDue(ctx, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("retry was immediately claimable: %+v", jobs)
	}
	jobs, err = store.ClaimDue(ctx, now.Add(time.Second), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Attempts != 1 {
		t.Fatalf("retry metadata %+v", jobs)
	}
	if jobs[0].LastError != "temporary downstream failure" {
		t.Fatalf("last error=%q", jobs[0].LastError)
	}
}

func TestRunnerStopsRetryingAfterMaximumAttempts(t *testing.T) {
	store, ctx, now := workerStore(t)
	if _, err := store.Enqueue(ctx, "incident.reported", []byte(`{}`), now); err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		Store: store,
		Clock: clock.Fixed{Current: now},
		Handler: HandlerFunc(func(context.Context, repository.OutboxJob) error {
			return errors.New("permanent rejection")
		}),
	}
	if err := runner.process(ctx, 1, 10); err != nil {
		t.Fatal(err)
	}
	jobs, err := store.ClaimDue(ctx, now.Add(24*time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("permanent job was reclaimed: %+v", jobs)
	}
}

func TestRunnerRunProcessesImmediatelyAndHonorsCancellation(t *testing.T) {
	store, ctx, now := workerStore(t)
	if _, err := store.Enqueue(ctx, "inspection.submitted", []byte(`{}`), now); err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	done := make(chan error, 1)
	runner := Runner{
		Store:       store,
		Clock:       clock.Fixed{Current: now},
		Interval:    time.Hour,
		MaxAttempts: 3,
		BatchSize:   10,
		Handler: HandlerFunc(func(context.Context, repository.OutboxJob) error {
			calls.Add(1)
			cancel()
			return nil
		}),
	}
	go func() { done <- runner.Run(runCtx) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
	if calls.Load() != 1 {
		t.Fatalf("handler calls=%d", calls.Load())
	}
}

func TestJSONHandlerAndDecodePayload(t *testing.T) {
	job := repository.OutboxJob{Type: "trip.completed", Payload: []byte(`{"trip_id":"t-1","distance":42}`)}
	type payload struct {
		TripID   string `json:"trip_id"`
		Distance int    `json:"distance"`
	}
	var decoded payload
	if err := DecodePayload(job, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.TripID != "t-1" || decoded.Distance != 42 {
		t.Fatalf("decoded payload %+v", decoded)
	}
	var gotType string
	var gotPayload map[string]any
	handler := JSONHandler(func(_ context.Context, typ string, data []byte) error {
		gotType = typ
		return json.Unmarshal(data, &gotPayload)
	})
	if err := handler.Handle(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if gotType != job.Type || gotPayload["trip_id"] != "t-1" {
		t.Fatalf("handler values type=%s payload=%+v", gotType, gotPayload)
	}
	if err := JSONHandler(nil).Handle(context.Background(), job); err == nil {
		t.Fatal("nil JSON handler should fail")
	}
	if err := DecodePayload(repository.OutboxJob{Payload: []byte("not-json")}, &decoded); err == nil {
		t.Fatal("invalid payload should fail")
	}
}
