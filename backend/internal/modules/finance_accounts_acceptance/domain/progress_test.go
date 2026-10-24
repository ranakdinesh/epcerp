package domain

import (
	"testing"
)

func TestProgressPreservesActualCompletionWithoutGrantingExecution(t *testing.T) {
	s := source()
	input := accepted()
	r, err := Apply(nil, owner(), caps(), Command{Action: "record", Reason: "Actual recorded completion", Acceptance: &input}, s, now(), identity)
	if err != nil {
		t.Fatal(err)
	}
	p := ProgressFact(&r, s.SourceFacts, true)
	if p.State != "completed" || !p.Current || p.Snapshot == nil {
		t.Fatal(p)
	}
	p.Snapshot.Source.Blockers = append(p.Snapshot.Source.Blockers, "changed")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("aliased reporting snapshot")
	}
	s.Available = false
	s.Blockers = []string{"Held project"}
	if ProgressFact(&r, s.SourceFacts, true).State != "completed" {
		t.Fatal("hold erased recorded completion")
	}
	if _, err := AcceptedFact(&r, s); err == nil {
		t.Fatal("reporting removed execution hold")
	}
	changed := s.SourceFacts
	changed.EngineerVersion++
	if p := ProgressFact(&r, changed, true); p.State != "blocked" || p.Current {
		t.Fatal("changed upstream earned completion", p)
	}
	if p := ProgressFact(&r, s.SourceFacts, false); p.State != "blocked" || p.Current {
		t.Fatal("missing upstream proof", p)
	}
	r.Acceptance.OfficerName = ""
	if p := ProgressFact(&r, s.SourceFacts, true); p.State != "blocked" || p.Snapshot != nil {
		t.Fatal("incomplete actual evidence earned completion", p)
	}
	r.Acceptance.Status = "pending"
	if p := ProgressFact(&r, s.SourceFacts, true); p.State != "not_started" || p.Snapshot != nil {
		t.Fatal("pending record earned completion", p)
	}
}
