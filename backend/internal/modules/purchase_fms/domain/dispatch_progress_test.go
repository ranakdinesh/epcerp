package domain

import (
	"testing"
)

func TestDispatchProgressPreservesActualCompletionWithoutGrantingExecution(t *testing.T) {
	s := dispatchSource()
	input := dispatchClear()
	r, err := DispatchApply(nil, dispatchOwner(), dispatchCaps(), DispatchCommand{Action: "record", Reason: "Actual recorded completion", Dispatch: &input}, s, dispatchNow(), dispatchIdentity)
	if err != nil {
		t.Fatal(err)
	}
	p := DispatchProgressFact(&r, s.DispatchSourceFacts, true)
	if p.State != "completed" || !p.Current || p.Snapshot == nil {
		t.Fatal(p)
	}
	p.Snapshot.Source.Blockers = append(p.Snapshot.Source.Blockers, "changed")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("aliased reporting snapshot")
	}
	s.Available = false
	s.Blockers = []string{"Held project"}
	if DispatchProgressFact(&r, s.DispatchSourceFacts, true).State != "completed" {
		t.Fatal("hold erased recorded completion")
	}
	if _, err := DispatchedFact(&r, s); err == nil {
		t.Fatal("reporting removed execution hold")
	}
	changed := s.DispatchSourceFacts
	changed.ClearanceVersion++
	if p := DispatchProgressFact(&r, changed, true); p.State != "blocked" || p.Current {
		t.Fatal("changed upstream earned completion", p)
	}
	if p := DispatchProgressFact(&r, s.DispatchSourceFacts, false); p.State != "blocked" || p.Current {
		t.Fatal("missing upstream proof", p)
	}
	r.Dispatch.LRNumber = ""
	if p := DispatchProgressFact(&r, s.DispatchSourceFacts, true); p.State != "blocked" || p.Snapshot != nil {
		t.Fatal("incomplete actual evidence earned completion", p)
	}
	r.Dispatch.Status = "pending"
	if p := DispatchProgressFact(&r, s.DispatchSourceFacts, true); p.State != "not_started" || p.Snapshot != nil {
		t.Fatal("pending record earned completion", p)
	}
}
