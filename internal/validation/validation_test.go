package validation

import "testing"

func TestNormalizePlateAndAcceptCommonFormats(t *testing.T) {
	for _, value := range []string{"沪A12345", "沪A·12345", "沪a-12345", "粤BD12345", "京A12345F"} {
		collector := Collector{}
		collector.Plate("plate_number", value)
		if err := collector.Err(); err != nil {
			t.Fatalf("plate %q rejected: %v", value, err)
		}
	}
	if got := NormalizePlate(" 沪a·12345 "); got != "沪A12345" {
		t.Fatalf("normalized plate = %q", got)
	}
}

func TestPlateRejectsNonStandardValues(t *testing.T) {
	for _, value := range []string{"123123", "沪环-001", "沪A1234", "沪A1234567"} {
		collector := Collector{}
		collector.Plate("plate_number", value)
		if collector.Err() == nil {
			t.Fatalf("plate %q was accepted", value)
		}
	}
}
