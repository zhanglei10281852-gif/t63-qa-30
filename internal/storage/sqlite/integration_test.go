package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"sanitation-operations/internal/apperror"
	"sanitation-operations/internal/domain/crew"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/domain/workplan"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
)

func openTestDB(t *testing.T) (*DB, context.Context, time.Time) {
	t.Helper()
	ctx := context.Background()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "operations.db"))
	db, err := Open(ctx, "file:"+path+"?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	return db, ctx, now
}

func TestMigrationNormalizesLegacyDemoPlates(t *testing.T) {
	ctx := context.Background()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "legacy.db"))
	db, err := Open(ctx, "file:"+path+"?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	legacy, err := vehicle.New("legacy-vehicle", "沪环-001", "compactor", "H-01", 9000, 100, now.AddDate(1, 0, 0), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveVehicle(ctx, legacy, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version=2"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, "file:"+path+"?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	result, err := reopened.ListVehicles(ctx, repository.VehicleFilter{Query: "沪A00001"}, pagination.Query{Limit: 10})
	if err != nil || result.Total != 1 || result.Items[0].PlateNumber != "沪A00001" {
		t.Fatalf("legacy plate was not normalized: result=%+v err=%v", result, err)
	}
}

func TestOpenAppliesSchemaAndSupportsRestartRecovery(t *testing.T) {
	ctx := context.Background()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "restart.db"))
	db, err := Open(ctx, "file:"+path+"?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	ids := &identity.Sequence{}
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	vehicleItem, err := vehicle.New(ids.NewID("vehicle"), "沪环-101", "compactor", "H-01", 9000, 100, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveVehicle(ctx, vehicleItem, 0); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, "file:"+path+"?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.GetVehicle(ctx, vehicleItem.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PlateNumber != vehicleItem.PlateNumber || loaded.Status != vehicle.Available {
		t.Fatalf("recovered value %+v", loaded)
	}
	var count int
	if err := reopened.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("migration rows = %d", count)
	}
}

func TestRepositoryPersistsRelationsAndReturnsIsolatedSlices(t *testing.T) {
	db, ctx, now := openTestDB(t)
	ids := &identity.Sequence{}
	driver, err := crew.New(ids.NewID("driver"), "DRV-101", "李师傅", "B2", now.AddDate(1, 0, 0), now)
	if err != nil {
		t.Fatal(err)
	}
	driver, err = driver.AddCertification(crew.Certification{ID: ids.NewID("cert"), DriverID: driver.ID, Code: "C-1", VehicleType: "compactor", ExpiresAt: now.AddDate(1, 0, 0)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveDriver(ctx, driver, 0); err != nil {
		t.Fatal(err)
	}
	loaded, err := db.GetDriver(ctx, driver.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Certifications) != 1 {
		t.Fatalf("certifications = %+v", loaded.Certifications)
	}
	loaded.Certifications[0].Code = "mutated"
	again, err := db.GetDriver(ctx, driver.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Certifications[0].Code != "C-1" {
		t.Fatalf("stored certification was mutated: %+v", again.Certifications)
	}

	route, err := workplan.NewRoute(ids.NewID("route"), "H-101", "北片区", "north", 5000, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	shift, err := workplan.NewShift(ids.NewID("shift"), route.ID, "2026-08-18", now.Add(time.Hour), now.Add(3*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveShift(ctx, shift, 0); err != nil {
		t.Fatal(err)
	}
	if result, err := db.ListShifts(ctx, repository.ShiftFilter{RouteID: route.ID}, pagination.Query{Limit: 10}); err != nil || result.Total != 1 {
		t.Fatalf("shifts = %+v, err=%v", result, err)
	}
}

func TestOptimisticVersionConflictIsReported(t *testing.T) {
	db, ctx, now := openTestDB(t)
	item, err := vehicle.New("v-conflict", "沪环-102", "sweeper", "H-01", 6500, 10, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveVehicle(ctx, item, 0); err != nil {
		t.Fatal(err)
	}
	updated, err := item.StartDispatch(now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveVehicle(ctx, updated, item.Version); err != nil {
		t.Fatal(err)
	}
	updated.Status = vehicle.Maintenance
	updated.Version++
	err = db.SaveVehicle(ctx, updated, item.Version)
	if err == nil {
		t.Fatal("stale version was accepted")
	}
	var app *apperror.AppError
	if !errors.As(err, &app) {
		t.Fatalf("expected app error, got %T: %v", err, err)
	}
}

func TestWithTxRollsBackAllWritesWhenCallbackFails(t *testing.T) {
	db, ctx, now := openTestDB(t)
	item, _ := vehicle.New("v-rollback", "沪环-103", "sweeper", "H-01", 6500, 10, now.Add(time.Hour), now)
	err := db.WithTx(ctx, func(ctx context.Context, tx repository.Tx) error {
		if err := tx.SaveVehicle(ctx, item, 0); err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatal("rollback callback unexpectedly succeeded")
	}
	if _, err := db.GetVehicle(ctx, item.ID); err == nil {
		t.Fatal("vehicle survived rollback")
	}
}

func TestForeignKeyAndUniqueConstraintsProtectData(t *testing.T) {
	db, ctx, now := openTestDB(t)
	item, _ := vehicle.New("v-unique", "沪环-104", "sweeper", "H-01", 6500, 10, now.Add(time.Hour), now)
	if err := db.SaveVehicle(ctx, item, 0); err != nil {
		t.Fatal(err)
	}
	duplicate, _ := vehicle.New("v-unique-2", item.PlateNumber, "sweeper", "H-01", 6500, 10, now.Add(time.Hour), now)
	if err := db.SaveVehicle(ctx, duplicate, 0); err == nil {
		t.Fatal("duplicate plate accepted")
	}
	badShift := workplan.Shift{ID: "orphan", RouteID: "missing", ServiceDate: "2026-08-18", StartAt: now, EndAt: now.Add(time.Hour), Status: workplan.Scheduled, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.SaveShift(ctx, badShift, 0); err == nil {
		t.Fatal("orphan shift accepted")
	}
}

func TestPingHonorsContextCancellation(t *testing.T) {
	db, _, _ := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := db.Ping(ctx); err == nil {
		t.Fatal("cancelled ping succeeded")
	}
}
