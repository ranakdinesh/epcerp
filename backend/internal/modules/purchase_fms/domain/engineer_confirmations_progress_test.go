package domain

import (
	"testing"
)

func TestEngineerConfirmationsProgressPreservesActualCompletionWithoutGrantingExecution(t *testing.T) {
	s := engineerConfirmationsSource()
	input := engineerConfirmationsConfirmed()
	r, err := EngineerConfirmationsApply(nil, engineerConfirmationsOwner(), engineerConfirmationsCaps(), EngineerConfirmationsCommand{Action: "record", Reason: "Actual recorded completion", Confirmation: &input}, s, engineerConfirmationsNow(), engineerConfirmationsIdentity)
	if err != nil {
		t.Fatal(err)
	}
	p := EngineerConfirmationsProgressFact(&r, s.EngineerConfirmationsSourceFacts, true)
	if p.State != "completed" || !p.Current || p.Snapshot == nil {
		t.Fatal(p)
	}
	p.Snapshot.Source.Blockers = append(p.Snapshot.Source.Blockers, "changed")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("aliased reporting snapshot")
	}
	s.Available = false
	s.Blockers = []string{"Held project"}
	if EngineerConfirmationsProgressFact(&r, s.EngineerConfirmationsSourceFacts, true).State != "completed" {
		t.Fatal("hold erased recorded completion")
	}
	if _, err := EngineerConfirmationsConfirmedFact(&r, s); err == nil {
		t.Fatal("reporting removed execution hold")
	}
	changed := s.EngineerConfirmationsSourceFacts
	changed.GRNVersion++
	if p := EngineerConfirmationsProgressFact(&r, changed, true); p.State != "blocked" || p.Current {
		t.Fatal("changed upstream earned completion", p)
	}
	if p := EngineerConfirmationsProgressFact(&r, s.EngineerConfirmationsSourceFacts, false); p.State != "blocked" || p.Current {
		t.Fatal("missing upstream proof", p)
	}
	r.Confirmation.EngineerName = ""
	if p := EngineerConfirmationsProgressFact(&r, s.EngineerConfirmationsSourceFacts, true); p.State != "blocked" || p.Snapshot != nil {
		t.Fatal("incomplete actual evidence earned completion", p)
	}
	r.Confirmation.Status = "pending"
	if p := EngineerConfirmationsProgressFact(&r, s.EngineerConfirmationsSourceFacts, true); p.State != "not_started" || p.Snapshot != nil {
		t.Fatal("pending record earned completion", p)
	}
}
