package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sanitation-operations/internal/businessday"
	"sanitation-operations/internal/clock"
	"sanitation-operations/internal/config"
	"sanitation-operations/internal/health"
	"sanitation-operations/internal/httpapi"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/middleware"
	"sanitation-operations/internal/observability"
	"sanitation-operations/internal/ratelimit"
	"sanitation-operations/internal/seed"
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
	"sanitation-operations/internal/storage/sqlite"
	"sanitation-operations/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger, err := observability.NewLogger(os.Stdout, cfg.LogLevel)
	if err != nil {
		return err
	}
	ctx := context.Background()
	db, err := sqlite.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	ids := identity.Random{}
	clk := clock.Real{}
	if err := seed.Ensure(ctx, db, ids, clk); err != nil {
		return err
	}
	authService := authservice.Service{Store: db, Clock: clk, IDs: ids, TTL: cfg.SessionTTL}
	if cfg.BootstrapPassword != "" {
		if err := authService.Bootstrap(ctx, cfg.BootstrapUsername, cfg.BootstrapPassword, cfg.BootstrapName); err != nil {
			return err
		}
	} else {
		logger.Warn("bootstrap administrator password is not configured")
	}
	dispatchService := dispatch.Service{Store: db, Clock: clk, IDs: ids}
	batchService := batch.Service{Dispatch: dispatchService, MaxParallel: 4}
	crewService := crewservice.Service{Store: db, Clock: clk, IDs: ids}
	fleetService := fleet.Service{Store: db, Clock: clk, IDs: ids}
	fuelService := fuelservice.Service{Store: db, Clock: clk, IDs: ids}
	inspectionService := inspectionservice.Service{Store: db, Clock: clk, IDs: ids}
	calendar, err := businessday.New(cfg.BusinessTimezone, 4)
	if err != nil {
		return err
	}
	planningService := planning.Service{Store: db, Clock: clk, IDs: ids, Calendar: &calendar}
	maintenanceService := maintservice.Service{Store: db, Clock: clk, IDs: ids}
	queryService := queryservice.Service{Store: db}
	reconciliationService := reconciliationservice.Service{Store: db, Calendar: calendar, Now: clk.Now}
	checker := health.Checker{Timeout: 2 * time.Second, Checks: map[string]health.Check{"database": db.Ping}}
	ready := func(ctx context.Context) error { _, err := checker.Run(ctx); return err }
	api := httpapi.Server{Auth: authService, Dispatch: dispatchService, Batch: batchService, Crew: crewService, Fleet: fleetService, Fuel: fuelService, Inspection: inspectionService, Planning: planningService, Maintenance: maintenanceService, Query: queryService, Reconciliation: reconciliationService, Clock: clk, Ready: ready}
	limiter := ratelimit.New(cfg.RateLimitCapacity, cfg.RateLimitPerSecond)
	handler := middleware.Chain(api.Handler(), middleware.RequestID(ids), middleware.Recover(logger), middleware.Logging(logger), middleware.CORS(cfg.AllowedOrigins), middleware.RateLimit(limiter), middleware.Authentication(authService.Authenticate, "/health/live", "/health/ready", "/api/v1/auth/login"), middleware.Timeout(30*time.Second))
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	runner := worker.Runner{Store: db, Clock: clk, Interval: cfg.WorkerInterval, MaxAttempts: 5, BatchSize: 20, Handler: worker.JSONHandler(func(ctx context.Context, typ string, payload []byte) error {
		logger.InfoContext(ctx, "outbox job handled", "type", typ, "bytes", len(payload))
		return nil
	}), Logger: logger}
	go func() {
		if err := runner.Run(workerCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("worker stopped", "error", err)
		}
	}()
	go func() {
		logger.Info("http server listening", "address", cfg.HTTPAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	stopWorker()
	return server.Shutdown(shutdown)
}
