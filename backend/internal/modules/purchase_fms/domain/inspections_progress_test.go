package domain

import (
	"testing"
)

func TestInspectionsInspectionProgressKeepsClearanceSeparateFromExecutionAuthority(t *testing.T) {
	s := inspectionsSource()
	q := inspectionsCompleted()
	r, err := InspectionsApply(nil, inspectionsOwner(), inspectionsCaps(), InspectionsCommand{Action: "record", Reason: "Record actual inspection", Inspection: &q}, s, inspectionsNow(), inspectionsIdentity)
	if err != nil {
		t.Fatal(err)
	}
	p := InspectionsProgressFact(&r, s.InspectionsSourceFacts, true)
	if p.State != "completed" || !p.Current || p.Snapshot == nil {
		t.Fatal(p)
	}
	p.Snapshot.Inspection.Points[0].Remarks = "changed"
	if r.Inspection.Points[0].Remarks == "changed" {
		t.Fatal("aliased witness evidence")
	}
	s.Available = false
	s.Blockers = []string{"Held project"}
	if InspectionsProgressFact(&r, s.InspectionsSourceFacts, true).State != "completed" {
		t.Fatal("hold erased inspection")
	}
	if _, err := InspectionsClearedFact(&r, s); err == nil {
		t.Fatal("report removed hold gate")
	}
	changed := s.InspectionsSourceFacts
	changed.TechnicalRevision++
	if p := InspectionsProgressFact(&r, changed, true); p.State != "blocked" || p.Current {
		t.Fatal("stale QAP earned progress", p)
	}
	if p := InspectionsProgressFact(&r, s.InspectionsSourceFacts, false); p.State != "blocked" || p.Current {
		t.Fatal("missing source proof", p)
	}
	r.Inspection.Points[0].Cleared = false
	if p := InspectionsProgressFact(&r, s.InspectionsSourceFacts, true); p.State != "blocked" || p.Snapshot != nil {
		t.Fatal("uncleared witness earned completion", p)
	}
}
func TestInspectionsInspectionProgressIntermediateAndRejectedStates(t *testing.T) {
	for _, status := range []string{"not_scheduled", "call_received", "scheduled", "rejected"} {
		t.Run(status, func(t *testing.T) {
			s := inspectionsSource()
			q := inspectionsCompleted()
			q.Status = status
			r, err := InspectionsApply(nil, inspectionsOwner(), inspectionsCaps(), InspectionsCommand{Action: "record", Reason: "Record actual inspection state", Inspection: &q}, s, inspectionsNow(), inspectionsIdentity)
			if err != nil {
				t.Fatal(err)
			}
			want := "in_progress"
			if status == "not_scheduled" {
				want = "not_started"
			}
			if status == "rejected" {
				want = "blocked"
			}
			if p := InspectionsProgressFact(&r, s.InspectionsSourceFacts, true); p.State != want || p.Snapshot != nil {
				t.Fatal(p)
			}
		})
	}
}
