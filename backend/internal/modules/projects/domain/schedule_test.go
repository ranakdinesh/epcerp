package domain

import (
	"testing"
)

func TestScheduleGapsAndRemainingBuffer(t *testing.T) {
	c := Capture{ScheduleStartBasis: "loi_date", LOIDate: "2026-10-01", PODate: "2026-10-02", PlannedStartDate: "2026-10-03", ActualStartDate: "2026-10-05", SafetyBufferPercent: "20"}
	r := c.Calculate(&WorkingCount{WorkingDays: 100})
	if *r.ContractualStartGap != 4 || *r.PlannedStartGap != 2 || *r.ReservedBufferDays != 20 || *r.ExecutionDays != 80 {
		t.Fatal(r)
	}
	c.ScheduleStartBasis = "po_date"
	if *c.Calculate(nil).ContractualStartGap != 3 {
		t.Fatal("PO not selected")
	}
	c.ActualStartDate = "2026-10-01"
	if *c.Calculate(nil).PlannedStartGap != -2 {
		t.Fatal("early start gap lost")
	}
	for _, tc := range []struct {
		percent                   string
		days, reserved, remaining int
	}{{"20.25", 100, 21, 79}, {"0", 22, 0, 22}, {"100", 22, 22, 0}, {"20", 0, 0, 0}, {"20", 22, 5, 17}} {
		c.SafetyBufferPercent = tc.percent
		r = c.Calculate(&WorkingCount{WorkingDays: tc.days})
		if *r.ReservedBufferDays != tc.reserved || *r.ExecutionDays != tc.remaining {
			t.Fatal("buffer rounding", tc)
		}
	}
	c.ActualStartDate = ""
	r = c.Calculate(nil)
	if r.CompletionDays != nil || r.ContractualStartGap != nil || r.ExecutionDays != nil {
		t.Fatal("pending values invented")
	}
	c = validCapture()
	c.ActualStartDate = "2026-10-05"
	c.ActualEndDate = "2026-10-04"
	if _, e := c.Validate("Project"); e == nil {
		t.Fatal("end before start accepted")
	}
}
