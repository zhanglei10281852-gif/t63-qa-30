package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"sanitation-operations/internal/clock"
	"sanitation-operations/internal/repository"
)

type Handler interface {
	Handle(context.Context, repository.OutboxJob) error
}
type HandlerFunc func(context.Context, repository.OutboxJob) error

func (f HandlerFunc) Handle(ctx context.Context, job repository.OutboxJob) error { return f(ctx, job) }

type Runner struct {
	Store       repository.Store
	Clock       clock.Clock
	Interval    time.Duration
	MaxAttempts int
	BatchSize   int
	Handler     Handler
	Logger      *slog.Logger
}

func (r Runner) Run(ctx context.Context) error {
	interval := r.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	batch := r.BatchSize
	if batch <= 0 {
		batch = 20
	}
	max := r.MaxAttempts
	if max <= 0 {
		max = 5
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	if err := r.process(ctx, max, batch); err != nil && ctx.Err() == nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.process(ctx, max, batch); err != nil && ctx.Err() == nil && r.Logger != nil {
				r.Logger.Error("worker cycle failed", "error", err)
			}
		}
	}
}

func (r Runner) process(ctx context.Context, max, batch int) error {
	jobs, err := r.Store.ClaimDue(ctx, r.Clock.Now(), batch)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := r.Handler.Handle(ctx, job); err != nil {
			if job.Attempts+1 >= max {
				if permanentErr := r.Store.MarkJobPermanent(ctx, job.ID, err.Error()); permanentErr != nil {
					return permanentErr
				}
			} else {
				delay := time.Duration(1<<min(job.Attempts, 6)) * time.Second
				if retryErr := r.Store.MarkJobRetry(ctx, job.ID, err.Error(), r.Clock.Now().Add(delay)); retryErr != nil {
					return retryErr
				}
			}
			continue
		}
		if err := r.Store.MarkJobDone(ctx, job.ID); err != nil {
			return err
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func JSONHandler(fn func(context.Context, string, []byte) error) HandlerFunc {
	return func(ctx context.Context, job repository.OutboxJob) error {
		if fn == nil {
			return fmt.Errorf("worker handler is nil")
		}
		return fn(ctx, job.Type, job.Payload)
	}
}
func DecodePayload(job repository.OutboxJob, target any) error {
	return json.Unmarshal(job.Payload, target)
}
