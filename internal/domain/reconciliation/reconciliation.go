package reconciliation

import (
	"sort"
	"time"

	"sanitation-operations/internal/domain/trip"
	"sanitation-operations/internal/domain/workplan"
)

type Severity string

const (
	Warning  Severity = "warning"
	Blocking Severity = "blocking"
)

type Finding struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	ShiftID  string   `json:"shift_id,omitempty"`
	TripID   string   `json:"trip_id,omitempty"`
	Message  string   `json:"message"`
}
type Report struct {
	ServiceDate     string    `json:"service_date"`
	ShiftCount      int       `json:"shift_count"`
	CompletedShifts int       `json:"completed_shifts"`
	TripCount       int       `json:"trip_count"`
	TotalDistanceKm int       `json:"total_distance_km"`
	Findings        []Finding `json:"findings"`
	Closable        bool      `json:"closable"`
	EvaluatedAt     time.Time `json:"evaluated_at"`
}

func Evaluate(serviceDate string, shifts []workplan.Shift, trips []trip.Trip, evaluated time.Time) Report {
	report := Report{ServiceDate: serviceDate, ShiftCount: len(shifts), TripCount: len(trips), Findings: []Finding{}, Closable: true, EvaluatedAt: evaluated.UTC()}
	tripsByShift := make(map[string][]trip.Trip, len(trips))
	for _, value := range trips {
		tripsByShift[value.ShiftID] = append(tripsByShift[value.ShiftID], value)
		if value.Status == trip.Completed {
			report.TotalDistanceKm += value.Distance()
		}
	}
	for _, shift := range shifts {
		values := tripsByShift[shift.ID]
		switch shift.Status {
		case workplan.Completed:
			report.CompletedShifts++
			if len(values) != 1 || values[0].Status != trip.Completed {
				report.add(Finding{Code: "completed_shift_trip_mismatch", Severity: Blocking, ShiftID: shift.ID, Message: "completed shift must have exactly one completed trip"})
			}
		case workplan.InProgress:
			if evaluated.After(shift.EndAt) {
				report.add(Finding{Code: "shift_overrun", Severity: Blocking, ShiftID: shift.ID, Message: "shift remains active after its planned end"})
			}
		case workplan.Assigned:
			if evaluated.After(shift.EndAt) {
				report.add(Finding{Code: "assigned_shift_not_started", Severity: Blocking, ShiftID: shift.ID, Message: "assigned shift did not start before its end"})
			}
		case workplan.Scheduled:
			if evaluated.After(shift.StartAt) {
				report.add(Finding{Code: "scheduled_shift_unassigned", Severity: Warning, ShiftID: shift.ID, Message: "scheduled shift reached its start without a vehicle"})
			}
		}
		if len(values) > 1 {
			report.add(Finding{Code: "multiple_trips_for_shift", Severity: Blocking, ShiftID: shift.ID, Message: "shift has more than one trip"})
		}
	}
	for _, value := range trips {
		if value.Status == trip.Active && value.StartedAt != nil && evaluated.Sub(*value.StartedAt) > 16*time.Hour {
			report.add(Finding{Code: "long_running_trip", Severity: Blocking, TripID: value.ID, ShiftID: value.ShiftID, Message: "trip has remained active beyond the operational limit"})
		}
		if value.Status == trip.Completed && value.Distance() == 0 {
			report.add(Finding{Code: "zero_distance_trip", Severity: Warning, TripID: value.ID, ShiftID: value.ShiftID, Message: "completed trip recorded zero distance"})
		}
	}
	sort.Slice(report.Findings, func(a, b int) bool {
		if report.Findings[a].Severity == report.Findings[b].Severity {
			return report.Findings[a].Code < report.Findings[b].Code
		}
		return report.Findings[a].Severity == Blocking
	})
	return report
}

func (r *Report) add(value Finding) {
	r.Findings = append(r.Findings, value)
	if value.Severity == Blocking {
		r.Closable = false
	}
}
