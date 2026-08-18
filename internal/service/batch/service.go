package batch

import (
	"context"
	"sync"

	"sanitation-operations/internal/service/dispatch"
)

type Assignment struct {
	ShiftID   string `json:"shift_id"`
	VehicleID string `json:"vehicle_id"`
}
type Result struct {
	ShiftID   string `json:"shift_id"`
	VehicleID string `json:"vehicle_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}
type Service struct {
	Dispatch    dispatch.Service
	MaxParallel int
}

func (s Service) Assign(ctx context.Context, assignments []Assignment, actor, request string) []Result {
	results := make([]Result, len(assignments))
	limit := s.MaxParallel
	if limit <= 0 {
		limit = 4
	}
	semaphore := make(chan struct{}, limit)
	var group sync.WaitGroup
	for index, assignment := range assignments {
		index, assignment := index, assignment
		group.Add(1)
		go func() {
			defer group.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results[index] = Result{ShiftID: assignment.ShiftID, VehicleID: assignment.VehicleID, Error: ctx.Err().Error()}
				return
			}
			_, err := s.Dispatch.AssignShift(ctx, dispatch.AssignInput{ShiftID: assignment.ShiftID, VehicleID: assignment.VehicleID, ActorID: actor, RequestID: request})
			results[index] = Result{ShiftID: assignment.ShiftID, VehicleID: assignment.VehicleID, Success: err == nil}
			if err != nil {
				results[index].Error = err.Error()
			}
		}()
	}
	group.Wait()
	return results
}
