package domain

import (
	"testing"
)

func TestClientClearanceProgressPreservesActualCompletionWithoutGrantingExecution(t *testing.T) {
	s := clientClearanceSource()
	input := clientClearanceClear()
	r, err := ClientClearanceApply(nil, clientClearanceOwner(), clientClearanceCaps(), ClientClearanceCommand{Action: "record", Reason: "Actual recorded completion", Clearance: &input}, s, clientClearanceNow(), clientClearanceIdentity)
	if err != nil {
		t.Fatal(err)
	}
	p := ClientClearanceProgressFact(&r, s.ClientClearanceSourceFacts, true)
	if p.State != "completed" || !p.Current || p.Snapshot == nil {
		t.Fatal(p)
	}
	p.Snapshot.Source.Blockers = append(p.Snapshot.Source.Blockers, "changed")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("aliased reporting snapshot")
	}
	s.Available = false
	s.Blockers = []string{"Held project"}
	if ClientClearanceProgressFact(&r, s.ClientClearanceSourceFacts, true).State != "completed" {
		t.Fatal("hold erased recorded completion")
	}
	if _, err := ClientClearanceClearedFact(&r, s); err == nil {
		t.Fatal("reporting removed execution hold")
	}
	changed := s.ClientClearanceSourceFacts
	changed.QualityVersion++
	if p := ClientClearanceProgressFact(&r, changed, true); p.State != "blocked" || p.Current {
		t.Fatal("changed upstream earned completion", p)
	}
	if p := ClientClearanceProgressFact(&r, s.ClientClearanceSourceFacts, false); p.State != "blocked" || p.Current {
		t.Fatal("missing upstream proof", p)
	}
	r.Clearance.Reference = ""
	if p := ClientClearanceProgressFact(&r, s.ClientClearanceSourceFacts, true); p.State != "blocked" || p.Snapshot != nil {
		t.Fatal("incomplete actual evidence earned completion", p)
	}
	r.Clearance.Status = "pending"
	if p := ClientClearanceProgressFact(&r, s.ClientClearanceSourceFacts, true); p.State != "not_started" || p.Snapshot != nil {
		t.Fatal("pending record earned completion", p)
	}
}
