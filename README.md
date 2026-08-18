# Sanitation Operations

Production-style Go backend for municipal sanitation fleet scheduling and execution. It coordinates vehicles, drivers, service routes, shifts, trips, inspections, maintenance, energy records, reconciliation, audit events, and an outbox worker.

## Architecture

- cmd/server: HTTP entrypoint and graceful shutdown
- internal/domain: state machines and value rules
- internal/service: transactional workflows and orchestration
- internal/repository: persistence contracts
- internal/storage/sqlite: SQLite repositories and migrations
- internal/httpapi: JSON endpoints and error contracts
- internal/middleware: authentication, request IDs, recovery, CORS, timeouts, and rate limits
- internal/worker: durable outbox processing with retry and permanent failure

The domain layer does not depend on HTTP or SQLite. HTTP handlers call services, and services use repository interfaces.

## Data Model

SQLite stores operators and sessions, vehicles, drivers and certifications, routes, shifts, trips, inspections and checklist items, maintenance orders, incidents, fuel logs, audit events, idempotency keys, and outbox jobs. Foreign keys, unique constraints, indexes, versions, and timestamps protect relationships and concurrent updates.

The embedded schema in internal/storage/sqlite/schema.sql creates a fresh database. Versioned SQL is also available under migrations/. Startup migration is idempotent and normalizes supported legacy demo plates without deleting historical records.

## Run

    cp .env.example .env
    go run ./cmd/server

Or use Docker:

    docker compose up --build

The server listens on http://localhost:8653 by default. Health endpoints are GET /health/live and GET /health/ready. The local bootstrap account is configured through environment variables; production deployments must supply their own secret.

## Configuration

- SANITATION_HTTP_ADDRESS
- SANITATION_DATABASE_URL
- SANITATION_ALLOWED_ORIGINS
- SANITATION_WORKER_INTERVAL
- SANITATION_BOOTSTRAP_ADMIN_USERNAME
- SANITATION_BOOTSTRAP_ADMIN_PASSWORD
- SANITATION_BOOTSTRAP_ADMIN_NAME

## Main API

Authentication uses /api/v1/auth/login, /api/v1/auth/me, and /api/v1/auth/logout. Fleet and execution resources are exposed under /api/v1/vehicles, /api/v1/drivers, /api/v1/routes, /api/v1/shifts, /api/v1/trips, /api/v1/inspections, /api/v1/maintenance, /api/v1/fuel, /api/v1/incidents, and /api/v1/reconciliation.

Errors use a stable JSON object containing code, message, and request_id. Request contexts propagate through services and repositories. Dispatch start, return, inspection submission, maintenance, and fuel workflows use transactions where multiple records must remain consistent.

## Test

    GOTOOLCHAIN=local go test ./... -count=1
    GOTOOLCHAIN=local go test -race ./... -count=1
    GOTOOLCHAIN=local go vet ./...
    GOTOOLCHAIN=local go build ./...
