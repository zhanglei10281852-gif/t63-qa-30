package schedule

import (
	"sort"
	"time"

	"sanitation-operations/internal/apperror"
)

type Window struct {
	ShiftID string    `json:"shift_id"`
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
}

func NewWindow(shiftID string, start, end time.Time) (Window, error) {
	if shiftID == "" || !end.After(start) {
		return Window{}, apperror.Validation(apperror.ErrValidation)
	}
	return Window{ShiftID: shiftID, Start: start.UTC(), End: end.UTC()}, nil
}
func (w Window) Duration() time.Duration { return w.End.Sub(w.Start) }
func (w Window) Overlaps(other Window) bool {
	return w.Start.Before(other.End) && other.Start.Before(w.End)
}
func (w Window) Gap(other Window) time.Duration {
	if w.Overlaps(other) {
		return 0
	}
	if w.End.Before(other.Start) {
		return other.Start.Sub(w.End)
	}
	return w.Start.Sub(other.End)
}

type Roster struct {
	VehicleID   string        `json:"vehicle_id"`
	ServiceDate string        `json:"service_date"`
	Windows     []Window      `json:"windows"`
	Duty        time.Duration `json:"duty"`
}

func NewRoster(vehicleID, serviceDate string) Roster {
	return Roster{VehicleID: vehicleID, ServiceDate: serviceDate, Windows: []Window{}}
}

func (r Roster) Add(window Window, maximumDuty, minimumBreak time.Duration) (Roster, error) {
	items := append([]Window(nil), r.Windows...)
	for _, existing := range items {
		if existing.ShiftID == window.ShiftID {
			continue
		}
		if existing.Overlaps(window) {
			return Roster{}, apperror.Conflict(apperror.ErrConflict)
		}
		gap := existing.Gap(window)
		if gap > 0 && gap < minimumBreak {
			return Roster{}, apperror.Conflict(apperror.ErrUnavailable)
		}
	}
	items = append(items, window)
	sort.Slice(items, func(a, b int) bool { return items[a].Start.Before(items[b].Start) })
	duty := time.Duration(0)
	for _, value := range items {
		duty += value.Duration()
	}
	if maximumDuty > 0 && duty > maximumDuty {
		return Roster{}, apperror.Conflict(apperror.ErrUnavailable)
	}
	r.Windows, r.Duty = items, duty
	return r, nil
}

func (r Roster) Clone() Roster { r.Windows = append([]Window(nil), r.Windows...); return r }
