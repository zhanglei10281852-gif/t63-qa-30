package sqlite

import (
	"context"
	"encoding/json"
	"time"

	"sanitation-operations/internal/audit"
	"sanitation-operations/internal/idempotency"
	"sanitation-operations/internal/repository"
)

func (s *queryStore) AppendAudit(ctx context.Context, event audit.Event) error {
	data, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	_, err = s.e.ExecContext(ctx, "INSERT INTO audit_events(id, actor_id, entity_type, entity_id, action, result, request_id, metadata_json, created_at) VALUES(?,?,?,?,?,?,?,?,?)", event.ID, event.ActorID, event.EntityType, event.EntityID, event.Action, event.Result, event.RequestID, string(data), formatTime(event.CreatedAt))
	return databaseError(err)
}

func (s *queryStore) FindIdempotency(ctx context.Context, scope, key string, now time.Time) (idempotency.Record, bool, error) {
	var r idempotency.Record
	var created, expires string
	err := s.e.QueryRowContext(ctx, "SELECT scope, key, response_code, response_json, created_at, expires_at FROM idempotency_keys WHERE scope=? AND key=? AND expires_at > ?", scope, key, formatTime(now)).Scan(&r.Scope, &r.Key, &r.Status, &r.Response, &created, &expires)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return idempotency.Record{}, false, nil
		}
		return idempotency.Record{}, false, err
	}
	r.CreatedAt, r.ExpiresAt = parseTime(created), parseTime(expires)
	return r, true, nil
}

func (s *queryStore) SaveIdempotency(ctx context.Context, r idempotency.Record) error {
	_, err := s.e.ExecContext(ctx, "INSERT INTO idempotency_keys(scope, key, response_code, response_json, created_at, expires_at) VALUES(?,?,?,?,?,?)", r.Scope, r.Key, r.Status, r.Response, formatTime(r.CreatedAt), formatTime(r.ExpiresAt))
	return databaseError(err)
}

func (s *queryStore) Enqueue(ctx context.Context, jobType string, payload []byte, due time.Time) (string, error) {
	id := s.ids.NewID("job")
	now := time.Now().UTC()
	_, err := s.e.ExecContext(ctx, "INSERT INTO outbox_jobs(id, job_type, payload_json, status, attempts, next_attempt_at, last_error, created_at, updated_at) VALUES(?,?,?,'pending',0,?,'',?,?)", id, jobType, string(payload), formatTime(due), formatTime(now), formatTime(now))
	return id, databaseError(err)
}

func (s *queryStore) ClaimDue(ctx context.Context, now time.Time, limit int) ([]repository.OutboxJob, error) {
	rows, err := s.e.QueryContext(ctx, "SELECT id, job_type, payload_json, attempts, next_attempt_at, last_error FROM outbox_jobs WHERE status='pending' AND next_attempt_at <= ? ORDER BY next_attempt_at ASC LIMIT ?", formatTime(now), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]repository.OutboxJob, 0, limit)
	for rows.Next() {
		var j repository.OutboxJob
		var payload, due string
		if err := rows.Scan(&j.ID, &j.Type, &payload, &j.Attempts, &due, &j.LastError); err != nil {
			return nil, err
		}
		j.Payload, j.NextAttemptAt = []byte(payload), parseTime(due)
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (s *queryStore) MarkJobDone(ctx context.Context, id string) error {
	_, err := s.e.ExecContext(ctx, "UPDATE outbox_jobs SET status='done', updated_at=? WHERE id=?", formatTime(time.Now().UTC()), id)
	return err
}
func (s *queryStore) MarkJobRetry(ctx context.Context, id, message string, next time.Time) error {
	_, err := s.e.ExecContext(ctx, "UPDATE outbox_jobs SET status='pending', attempts=attempts+1, next_attempt_at=?, last_error=?, updated_at=? WHERE id=?", formatTime(next), message, formatTime(time.Now().UTC()), id)
	return err
}
func (s *queryStore) MarkJobPermanent(ctx context.Context, id, message string) error {
	_, err := s.e.ExecContext(ctx, "UPDATE outbox_jobs SET status='permanent_failure', attempts=attempts+1, last_error=?, updated_at=? WHERE id=?", message, formatTime(time.Now().UTC()), id)
	return err
}
