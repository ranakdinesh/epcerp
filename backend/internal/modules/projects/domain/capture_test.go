package domain

import (
	"testing"
)

func validCapture() Capture {
	return Capture{ProjectType: "electrical", SiteState: "Delhi", SiteCity: "New Delhi", OperationsManagerID: "11111111-1111-4111-8111-111111111111", ContractValue: "99999999999999.99", Currency: "INR", ScheduleStartBasis: "loi_date", LOIDate: "2026-10-01", PlannedDurationDays: "31", SafetyBufferPercent: "20.25", SafetyBufferDays: "5"}
}
func TestProjectCaptureDatesAndMoney(t *testing.T) {
	c, e := validCapture().Validate("Project A")
	if e != nil || c.PlannedStartDate != "2026-10-01" || c.PlannedCompletionDate != "2026-10-31" || c.ShortName != "Project A" {
		t.Fatal(c, e)
	}
	original := validCapture()
	original.PlannedStartDate = "2026-10-04"
	original.PlannedCompletionDate = "2026-11-15"
	c, e = original.Validate("Project A")
	if e != nil || c.PlannedCompletionDate != "2026-11-15" || c.PlannedStartDate != "2026-10-04" {
		t.Fatal("explicit dates were overwritten", e)
	}
	tests := []struct {
		name   string
		change func(*Capture)
	}{
		{"fraction precision", func(c *Capture) { c.ContractValue = "1.001" }}, {"amount overflow", func(c *Capture) { c.ContractValue = "100000000000000" }}, {"negative value", func(c *Capture) { c.ContractValue = "-1" }}, {"missing amount", func(c *Capture) { c.ContractValue = "" }}, {"leap date", func(c *Capture) { c.LOIDate = "2025-02-29" }}, {"missing milestone", func(c *Capture) { c.ScheduleStartBasis = "po_date" }}, {"unknown basis", func(c *Capture) { c.ScheduleStartBasis = "unknown" }}, {"short duration", func(c *Capture) { c.PlannedDurationDays = "0" }}, {"long duration", func(c *Capture) { c.PlannedDurationDays = "36501" }}, {"buffer", func(c *Capture) { c.SafetyBufferPercent = "100.01" }}, {"early finish", func(c *Capture) { c.PlannedCompletionDate = "2026-09-30" }}, {"early handover", func(c *Capture) { c.SiteHandoverDate = "2026-10-15" }}, {"missing location", func(c *Capture) { c.SiteCity = "" }}, {"invalid identity", func(c *Capture) { c.OperationsManagerID = "user" }}, {"unsupported type", func(c *Capture) { c.ProjectType = "other" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validCapture()
			tc.change(&c)
			if _, e := c.Validate("Project A"); e == nil {
				t.Fatal("invalid capture accepted")
			}
		})
	}
}

func TestDraftCaptureAllowsUnfinishedFieldsAndRejectsInvalidValues(t *testing.T) {
	if _, err := (Capture{Currency: "INR", ProjectType: "electrical"}).ValidateDraft(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []Capture{
		{ContractValue: "-1"}, {ProjectType: "unsupported"},
		{LOIDate: "2026-02-30"}, {PlannedDurationDays: "0"},
		{SafetyBufferPercent: "101"}, {OperationsManagerID: "invalid"},
	} {
		if _, err := c.ValidateDraft(); err == nil {
			t.Fatal("invalid entered draft value accepted")
		}
	}
	// Schedule preview receives dates, not a full project form.
	c, err := (Capture{LOIDate: "2026-10-01", PlannedCompletionDate: "2026-10-31", ScheduleStartBasis: "loi_date"}).ValidateDraft()
	if err != nil || c.PlannedStartDate != "2026-10-01" {
		t.Fatal("schedule-only capture", err)
	}
}
