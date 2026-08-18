package seed

import (
	"context"
	"time"

	"sanitation-operations/internal/clock"
	"sanitation-operations/internal/domain/crew"
	"sanitation-operations/internal/domain/vehicle"
	"sanitation-operations/internal/domain/workplan"
	"sanitation-operations/internal/identity"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
)

func Ensure(ctx context.Context, store repository.Store, ids identity.Generator, clk clock.Clock) error {
	vehicles, err := store.ListVehicles(ctx, repository.VehicleFilter{}, pagination.Query{Limit: 1})
	if err != nil {
		return err
	}
	now := clk.Now()
	if vehicles.Total == 0 {
		for _, value := range []struct {
			plate, kind, depot string
			capacity           int
		}{{"沪A00001", "compactor", "H-01", 9000}, {"沪A00002", "sweeper", "H-01", 6500}} {
			item, err := vehicle.New(ids.NewID("vehicle"), value.plate, value.kind, value.depot, value.capacity, 12000, now.Add(30*24*time.Hour), now)
			if err != nil {
				return err
			}
			if err := store.SaveVehicle(ctx, item, 0); err != nil {
				return err
			}
		}
	}
	routes, err := store.ListRoutes(ctx, pagination.Query{Limit: 1})
	if err != nil {
		return err
	}
	if routes.Total == 0 {
		route, err := workplan.NewRoute(ids.NewID("route"), "H-001", "静安北片区清运线", "Jingan-North", 5000, now)
		if err != nil {
			return err
		}
		if err := store.SaveRoute(ctx, route); err != nil {
			return err
		}
	}
	drivers, err := store.ListDrivers(ctx, "", pagination.Query{Limit: 1})
	if err != nil {
		return err
	}
	if drivers.Total == 0 {
		driver, err := crew.New(ids.NewID("driver"), "DRV-001", "示范驾驶员", "B2", now.AddDate(1, 0, 0), now)
		if err != nil {
			return err
		}
		for _, vehicleType := range []string{"compactor", "sweeper"} {
			driver, err = driver.AddCertification(crew.Certification{ID: ids.NewID("cert"), DriverID: driver.ID, Code: "CERT-" + vehicleType, VehicleType: vehicleType, ExpiresAt: now.AddDate(1, 0, 0)}, now)
			if err != nil {
				return err
			}
		}
		if err := store.SaveDriver(ctx, driver, 0); err != nil {
			return err
		}
	}
	return nil
}
