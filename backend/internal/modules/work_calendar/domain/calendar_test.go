package domain

import (
	"testing"
)

func TestWorkingCalendar(t *testing.T) {
	for _, tc := range []struct {
		start, end string
		holidays   map[string]bool
		want       Count
	}{
		{"2026-10-05", "2026-10-31", nil, Count{27, 5, 0, 22}},
		{"2026-10-05", "2026-10-31", map[string]bool{"2026-10-06": true, "2026-10-11": true, "2026-10-07": false}, Count{27, 5, 1, 21}},
		{"2024-02-28", "2024-03-01", nil, Count{3, 0, 0, 3}},
		{"2026-10-10", "2026-10-10", nil, Count{1, 1, 0, 0}},
		{"2026-10-24", "2026-10-24", nil, Count{1, 1, 0, 0}},
		{"2026-10-31", "2026-10-31", nil, Count{1, 0, 0, 1}},
		{"2026-10-05", "2026-10-04", nil, Count{}},
	} {
		got, e := Working(tc.start, tc.end, tc.holidays)
		if e != nil || got != tc.want {
			t.Fatalf("%s..%s: %+v %v", tc.start, tc.end, got, e)
		}
	}
	for _, dates := range [][2]string{{"2025-02-29", "2026-10-01"}, {"0000-01-01", "2026-10-01"}, {"1900-01-01", "2100-01-01"}} {
		if _, e := Working(dates[0], dates[1], nil); e == nil {
			t.Fatal("invalid range accepted")
		}
	}
}
