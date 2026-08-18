package scheduling

import (
	"context"
	"time"

	"sanitation-operations/internal/domain/schedule"
	"sanitation-operations/internal/domain/workplan"
	"sanitation-operations/internal/pagination"
	"sanitation-operations/internal/repository"
)

type Checker struct {
	MaximumDuty  time.Duration
	MinimumBreak time.Duration
}

func (c Checker) CanAssign(ctx context.Context, reader repository.ShiftReader, target workplan.Shift, vehicleID string) error {
	roster := schedule.NewRoster(vehicleID, target.ServiceDate)
	offset := 0
	for {
		page, err := reader.ListShifts(ctx, repository.ShiftFilter{ServiceDate: target.ServiceDate}, pagination.Query{Limit: 100, Offset: offset})
		if err != nil {
			return err
		}
		for _, existing := range page.Items {
			if existing.AssignedVehicleID == nil || *existing.AssignedVehicleID != vehicleID || existing.Status == workplan.Cancelled {
				continue
			}
			window, err := schedule.NewWindow(existing.ID, existing.StartAt, existing.EndAt)
			if err != nil {
				return err
			}
			roster, err = roster.Add(window, c.maximumDuty(), c.minimumBreak())
			if err != nil {
				return err
			}
		}
		offset += len(page.Items)
		if offset >= page.Total {
			break
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	window, err := schedule.NewWindow(target.ID, target.StartAt, target.EndAt)
	if err != nil {
		return err
	}
	_, err = roster.Add(window, c.maximumDuty(), c.minimumBreak())
	return err
}

func (c Checker) maximumDuty() time.Duration {
	if c.MaximumDuty <= 0 {
		return 16 * time.Hour
	}
	return c.MaximumDuty
}
func (c Checker) minimumBreak() time.Duration {
	if c.MinimumBreak <= 0 {
		return 15 * time.Minute
	}
	return c.MinimumBreak
}
