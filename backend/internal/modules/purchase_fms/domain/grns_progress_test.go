package domain

import (
	"testing"
)

func TestGRNsProgressPreservesActualCompletionWithoutGrantingExecution(t *testing.T) {
	s := gRNsSource()
	input := gRNsIssued()
	r, err := GRNsApply(nil, gRNsOwner(), gRNsCaps(), GRNsCommand{Action: "record", Reason: "Actual recorded completion", GRN: &input}, s, gRNsNow(), gRNsIdentity)
	if err != nil {
		t.Fatal(err)
	}
	p := GRNsProgressFact(&r, s.GRNsSourceFacts, true)
	if p.State != "completed" || !p.Current || p.Snapshot == nil {
		t.Fatal(p)
	}
	p.Snapshot.Source.Blockers = append(p.Snapshot.Source.Blockers, "changed")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("aliased reporting snapshot")
	}
	s.Available = false
	s.Blockers = []string{"Held project"}
	if GRNsProgressFact(&r, s.GRNsSourceFacts, true).State != "completed" {
		t.Fatal("hold erased recorded completion")
	}
	if _, err := GRNsIssuedFact(&r, s); err == nil {
		t.Fatal("reporting removed execution hold")
	}
	changed := s.GRNsSourceFacts
	changed.ReceiptVersion++
	if p := GRNsProgressFact(&r, changed, true); p.State != "blocked" || p.Current {
		t.Fatal("changed upstream earned completion", p)
	}
	if p := GRNsProgressFact(&r, s.GRNsSourceFacts, false); p.State != "blocked" || p.Current {
		t.Fatal("missing upstream proof", p)
	}
	r.GRN.Number = ""
	if p := GRNsProgressFact(&r, s.GRNsSourceFacts, true); p.State != "blocked" || p.Snapshot != nil {
		t.Fatal("incomplete actual evidence earned completion", p)
	}
	r.GRN.Status = "pending"
	if p := GRNsProgressFact(&r, s.GRNsSourceFacts, true); p.State != "not_started" || p.Snapshot != nil {
		t.Fatal("pending record earned completion", p)
	}
}

func TestGRNsIssuedGRNNeedsWarrantyReferenceForProgressCompletion(t *testing.T) {
	s := gRNsSource()
	q := gRNsIssued()
	q.WarrantyReference = ""
	r, err := GRNsApply(nil, gRNsOwner(), gRNsCaps(), GRNsCommand{Action: "record", Reason: "Actual issued GRN, warranty pending", GRN: &q}, s, gRNsNow(), gRNsIdentity)
	if err != nil {
		t.Fatal("optional issuance field became compulsory", err)
	}
	if _, err := GRNsIssuedFact(&r, s); err != nil {
		t.Fatal("issued GRN prerequisite changed", err)
	}
	p := GRNsProgressFact(&r, s.GRNsSourceFacts, true)
	if p.State != "in_progress" || !p.Current || p.Snapshot == nil || p.Snapshot.GRN.Status != "issued" {
		t.Fatal("missing warranty counted as complete", p)
	}
}
