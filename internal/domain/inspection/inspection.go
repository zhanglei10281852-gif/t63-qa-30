package inspection

import (
	"sort"
	"time"

	"sanitation-operations/internal/apperror"
)

type Status string

const (
	Draft     Status = "draft"
	Submitted Status = "submitted"
	Passed    Status = "passed"
	Failed    Status = "failed"
)

type Result string

const (
	ResultPass Result = "pass"
	ResultFail Result = "fail"
	ResultNA   Result = "not_applicable"
)

type Item struct {
	ID           string `json:"id"`
	InspectionID string `json:"inspection_id"`
	Code         string `json:"code"`
	Result       Result `json:"result"`
	Notes        string `json:"notes"`
}

type Inspection struct {
	ID          string    `json:"id"`
	VehicleID   string    `json:"vehicle_id"`
	Inspector   string    `json:"inspector"`
	Status      Status    `json:"status"`
	InspectedAt time.Time `json:"inspected_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Score       int       `json:"score"`
	Version     int       `json:"version"`
	Items       []Item    `json:"items"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

var RequiredItems = []string{"brakes", "lights", "hydraulics", "tires", "safety_kit"}

func New(id, vehicleID, inspector string, inspectedAt, expiresAt, now time.Time) (Inspection, error) {
	if id == "" || vehicleID == "" || inspector == "" || expiresAt.Before(inspectedAt) {
		return Inspection{}, apperror.Validation(apperror.ErrValidation)
	}
	return Inspection{ID: id, VehicleID: vehicleID, Inspector: inspector, Status: Draft, InspectedAt: inspectedAt.UTC(), ExpiresAt: expiresAt.UTC(), Version: 1, Items: []Item{}, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (i Inspection) Record(item Item, now time.Time) (Inspection, error) {
	if i.Status != Draft || !required(item.Code) || (item.Result != ResultPass && item.Result != ResultFail && item.Result != ResultNA) {
		return Inspection{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	copyItems := append([]Item(nil), i.Items...)
	found := false
	for index := range copyItems {
		if copyItems[index].Code == item.Code {
			copyItems[index] = item
			found = true
			break
		}
	}
	if !found {
		copyItems = append(copyItems, item)
	}
	sort.Slice(copyItems, func(a, b int) bool { return copyItems[a].Code < copyItems[b].Code })
	i.Items = copyItems
	i.UpdatedAt = now.UTC()
	return i, nil
}

func (i Inspection) Submit(now time.Time) (Inspection, error) {
	if i.Status != Draft || len(i.Items) != len(RequiredItems) {
		return Inspection{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	passed := 0
	for _, code := range RequiredItems {
		item, ok := i.item(code)
		if !ok {
			return Inspection{}, apperror.Conflict(apperror.ErrInvalidState)
		}
		if item.Result == ResultPass || item.Result == ResultNA {
			passed++
		}
	}
	i.Status = Submitted
	i.Score = passed * 100 / len(RequiredItems)
	if i.Score == 100 {
		i.Status = Passed
	} else {
		i.Status = Failed
	}
	i.Version++
	i.UpdatedAt = now.UTC()
	return i, nil
}

func (i Inspection) Clone() Inspection { i.Items = append([]Item(nil), i.Items...); return i }

func (i Inspection) item(code string) (Item, bool) {
	for _, value := range i.Items {
		if value.Code == code {
			return value, true
		}
	}
	return Item{}, false
}
func required(code string) bool {
	for _, value := range RequiredItems {
		if value == code {
			return true
		}
	}
	return false
}
