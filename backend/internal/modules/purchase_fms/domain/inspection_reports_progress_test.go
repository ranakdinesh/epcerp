package domain

import (
	"testing"
)

func TestInspectionReportsReportProgressRequiresCurrentActualIssuance(t *testing.T) {
	s := inspectionReportsSource()
	q := inspectionReportsIssued()
	r, err := InspectionReportsApply(nil, inspectionReportsOwner(), inspectionReportsCaps(), InspectionReportsCommand{Action: "record", Reason: "Actual IRN issued", Report: &q}, s, inspectionReportsNow(), inspectionReportsIdentity)
	if err != nil {
		t.Fatal(err)
	}
	p := InspectionReportsProgressFact(&r, s.InspectionReportsSourceFacts, true)
	if p.State != "completed" || !p.Current || p.Snapshot == nil || p.Snapshot.Report.Quantity != "5.000001" {
		t.Fatal(p)
	}
	p.Snapshot.Source.Blockers = append(p.Snapshot.Source.Blockers, "changed")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("aliased report")
	}
	s.Available = false
	s.Blockers = []string{"Held project"}
	if InspectionReportsProgressFact(&r, s.InspectionReportsSourceFacts, true).State != "completed" {
		t.Fatal("hold erased issued report")
	}
	if _, err := InspectionReportsIssuedFact(&r, s); err == nil {
		t.Fatal("report bypassed hold")
	}
	changed := s.InspectionReportsSourceFacts
	changed.InspectionRevision++
	if p := InspectionReportsProgressFact(&r, changed, true); p.State != "blocked" || p.Current {
		t.Fatal("obsolete inspection earned issuance", p)
	}
	if p := InspectionReportsProgressFact(&r, s.InspectionReportsSourceFacts, false); p.State != "blocked" || p.Current {
		t.Fatal("missing upstream proof", p)
	}
	r.Report.IRNNumber = ""
	if p := InspectionReportsProgressFact(&r, s.InspectionReportsSourceFacts, true); p.State != "blocked" || p.Snapshot != nil {
		t.Fatal("missing IRN number earned issuance", p)
	}
	r.Report.Status = "pending"
	if p := InspectionReportsProgressFact(&r, s.InspectionReportsSourceFacts, true); p.State != "not_started" || p.Snapshot != nil {
		t.Fatal(p)
	}
}
