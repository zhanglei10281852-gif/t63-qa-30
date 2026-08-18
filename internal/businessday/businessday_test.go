package businessday

import (
	"testing"
	"time"
)

func TestShanghaiServiceDateUsesFourAMCutoff(t *testing.T) {
	calendar, err := New("Asia/Shanghai", 4)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, value, want string }{
		{"before cutoff", "2026-08-18T03:59:59+08:00", "2026-08-17"},
		{"at cutoff", "2026-08-18T04:00:00+08:00", "2026-08-18"},
		{"daytime", "2026-08-18T12:00:00+08:00", "2026-08-18"},
		{"next day before cutoff", "2026-08-19T01:00:00+08:00", "2026-08-18"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			value, err := time.Parse(time.RFC3339, test.value)
			if err != nil {
				t.Fatal(err)
			}
			if got := calendar.ServiceDate(value); got != test.want {
				t.Fatalf("ServiceDate() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestCalendarBoundsAreUtcAndTwentyFourHours(t *testing.T) {
	calendar, _ := New("Asia/Shanghai", 4)
	start, end, err := calendar.Bounds("2026-08-18")
	if err != nil {
		t.Fatal(err)
	}
	if start.Format(time.RFC3339) != "2026-08-17T20:00:00Z" {
		t.Fatalf("unexpected start %s", start.Format(time.RFC3339))
	}
	if end.Sub(start) != 24*time.Hour {
		t.Fatalf("window is %s", end.Sub(start))
	}
	if _, _, err := calendar.Bounds("not-a-date"); err == nil {
		t.Fatal("invalid date should fail")
	}
}

func TestValidateWindowRejectsCrossBusinessDayShift(t *testing.T) {
	calendar, _ := New("Asia/Shanghai", 4)
	start, _ := calendar.ParseLocal("2026-08-18", "23:00")
	end, _ := calendar.ParseLocal("2026-08-19", "04:01")
	if err := calendar.ValidateWindow("2026-08-18", start, end); err == nil {
		t.Fatal("cross-day shift should fail")
	}
	validEnd, _ := calendar.ParseLocal("2026-08-19", "03:59")
	if err := calendar.ValidateWindow("2026-08-18", start, validEnd); err != nil {
		t.Fatalf("valid boundary rejected: %v", err)
	}
}

func TestCalendarRejectsInvalidCutoffAndParsesLocalTime(t *testing.T) {
	if _, err := New("Asia/Shanghai", -1); err == nil {
		t.Fatal("negative cutoff should fail")
	}
	if _, err := New("Asia/Shanghai", 24); err == nil {
		t.Fatal("cutoff after a day should fail")
	}
	calendar, _ := New("Asia/Shanghai", 4)
	value, err := calendar.ParseLocal("2026-08-18", "04:15")
	if err != nil {
		t.Fatal(err)
	}
	if value.Format(time.RFC3339) != "2026-08-17T20:15:00Z" {
		t.Fatalf("unexpected parsed value %s", value.Format(time.RFC3339))
	}
	if _, err := calendar.ParseLocal("bad", "time"); err == nil {
		t.Fatal("invalid local time should fail")
	}
	next, err := calendar.Next("2026-08-18")
	if err != nil || next != "2026-08-19" {
		t.Fatalf("next = %s, %v", next, err)
	}
}
