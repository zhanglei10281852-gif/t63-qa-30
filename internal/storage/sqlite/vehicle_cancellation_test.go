package sqlite

import (
	"context"
	"testing"

	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
)

func TestVehicleListingStopsAfterCancellation(t *testing.T) {
	db, ctx, now := openTestDB(t)
	item, _ := vehicle.New("cancel-1", "沪A91001", "sweeper", "H-01", 6000, 10, now.AddDate(1, 0, 0), now)
	if err := db.SaveVehicle(ctx, item, 0); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := db.ListVehicles(cancelled, repository.VehicleFilter{}, pagination.Query{Limit: 10}); err == nil {
		t.Fatalf("cancelled listing returned %+v", result)
	}
}
