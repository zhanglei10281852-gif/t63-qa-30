package inspection_test

import (
	"testing"
	"time"

	"sanitation-operations/internal/domain/inspection"
)

var base = time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)

func draft(t *testing.T) inspection.Inspection {
	t.Helper()
	value, err := inspection.New("inspection-1", "vehicle-1", "inspector", base, base.Add(30*24*time.Hour), base)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func fill(t *testing.T, value inspection.Inspection, failing string) inspection.Inspection {
	t.Helper()
	var err error
	for index, code := range inspection.RequiredItems {
		result := inspection.ResultPass
		if code == failing {
			result = inspection.ResultFail
		}
		value, err = value.Record(inspection.Item{ID: "item-" + string(rune('a'+index)), InspectionID: value.ID, Code: code, Result: result}, base.Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
	}
	return value
}

func TestInspectionPassesWhenEveryRequiredItemPasses(t *testing.T) {
	value := fill(t, draft(t), "")
	submitted, err := value.Submit(base.Add(2 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Status != inspection.Passed || submitted.Score != 100 || submitted.Version != 2 {
		t.Fatalf("unexpected: %+v", submitted)
	}
}
func TestInspectionFailsWhenOneRequiredItemFails(t *testing.T) {
	value := fill(t, draft(t), "brakes")
	submitted, err := value.Submit(base.Add(2 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Status != inspection.Failed || submitted.Score != 80 {
		t.Fatalf("unexpected: %+v", submitted)
	}
}
func TestInspectionCannotSubmitIncompleteChecklist(t *testing.T) {
	value := draft(t)
	updated, err := value.Record(inspection.Item{ID: "one", InspectionID: value.ID, Code: "brakes", Result: inspection.ResultPass}, base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := updated.Submit(base); err == nil {
		t.Fatal("expected incomplete checklist error")
	}
}
func TestRecordingSameItemReplacesResultWithoutDuplicating(t *testing.T) {
	value := draft(t)
	first := inspection.Item{ID: "one", InspectionID: value.ID, Code: "brakes", Result: inspection.ResultFail}
	value, _ = value.Record(first, base)
	second := inspection.Item{ID: "two", InspectionID: value.ID, Code: "brakes", Result: inspection.ResultPass}
	value, _ = value.Record(second, base)
	if len(value.Items) != 1 || value.Items[0].ID != "two" || value.Items[0].Result != inspection.ResultPass {
		t.Fatalf("unexpected items: %+v", value.Items)
	}
}
func TestInspectionRejectsUnknownChecklistCode(t *testing.T) {
	value := draft(t)
	if _, err := value.Record(inspection.Item{ID: "x", InspectionID: value.ID, Code: "unknown", Result: inspection.ResultPass}, base); err == nil {
		t.Fatal("expected unknown code error")
	}
}
func TestInspectionCloneIsolatesItems(t *testing.T) {
	value := draft(t)
	value, _ = value.Record(inspection.Item{ID: "one", InspectionID: value.ID, Code: "brakes", Result: inspection.ResultPass}, base)
	clone := value.Clone()
	clone.Items[0].Notes = "changed"
	if value.Items[0].Notes == "changed" {
		t.Fatal("clone shares item slice")
	}
}
func TestInspectionRejectsExpiryBeforeInspection(t *testing.T) {
	if _, err := inspection.New("id", "vehicle", "person", base, base.Add(-time.Hour), base); err == nil {
		t.Fatal("expected expiry validation")
	}
}
