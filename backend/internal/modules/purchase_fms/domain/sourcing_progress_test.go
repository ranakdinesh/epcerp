package domain

import (
	"testing"
)

func sourcingProgressCase() (SourcingCase, SourcingSource) {
	_, _, s, _, _, now := sourcingSetup()
	offer := SourcingOffer{ID: sourcingOfferID, SupplierID: sourcingSupplier, UnitPrice: "10", Discount: "0", Tax: "0", Freight: "0", Other: "0", Active: true, SentDate: SourcingToday(now), Channel: "manual", DispatchReference: "Actual RFQ dispatch"}
	c := SourcingCase{Active: true, State: "released", ID: sourcingCaseID, ProjectID: sourcingProjectID, PackageID: sourcingPackageID, Source: s, Version: 7, ApproverID: sourcingApprover, ApprovedBy: sourcingApprover, ApprovedAt: &now, ProposedBy: sourcingOperator, ReleasedAt: &now, LOIReference: "LOI-actual", LOISentDate: SourcingToday(now), LOIChannel: "manual", LOIDispatchReference: "Actual LOI dispatch", Offers: []SourcingOffer{offer}, Decision: &SourcingDecision{Source: s, Offer: offer, FinancialReference: "approved:fingerprint"}}
	return c, s
}
func TestSourcingSourcingProgressPreservesReleasedEvidenceAndExactDemand(t *testing.T) {
	c, s := sourcingProgressCase()
	p := SourcingProgressFacts(&c, sourcingProjectID, sourcingPackageID, s, true)
	if !p.Current || p.Snapshot == nil {
		t.Fatal(p)
	}
	for _, step := range p.Steps {
		if step.State != "completed" {
			t.Fatal(step)
		}
	}
	p.Snapshot.Source.Blockers = append(p.Snapshot.Source.Blockers, "changed")
	p.Snapshot.Offer.UnitPrice = "999"
	if len(c.Decision.Source.Blockers) != 0 || c.Decision.Offer.UnitPrice != "10" {
		t.Fatal("aliased LOI snapshot")
	}
	s.Available = false
	s.Blockers = []string{"Held project"}
	if p := SourcingProgressFacts(&c, sourcingProjectID, sourcingPackageID, s, true); p.Steps[2].State != "completed" {
		t.Fatal("hold erased issued LOI", p)
	}
	if s.Guard() == nil {
		t.Fatal("reporting relaxed source execution gate")
	}
	s.DemandVersion++
	if p := SourcingProgressFacts(&c, sourcingProjectID, sourcingPackageID, s, true); p.Current || p.Steps[2].State != "blocked" {
		t.Fatal("obsolete demand earned LOI", p)
	}
}
func TestSourcingSourcingProgressNeverInfersLOIFromSelection(t *testing.T) {
	c, s := sourcingProgressCase()
	c.State = "approved"
	p := SourcingProgressFacts(&c, sourcingProjectID, sourcingPackageID, s, true)
	if p.Steps[1].State != "completed" || p.Steps[2].State != "in_progress" || p.Snapshot != nil {
		t.Fatal("selection invented release", p)
	}
	c.State = "released"
	c.LOIDispatchReference = ""
	if p := SourcingProgressFacts(&c, sourcingProjectID, sourcingPackageID, s, true); p.Steps[2].State != "blocked" || p.Snapshot != nil {
		t.Fatal("missing LOI dispatch earned completion", p)
	}
	c, s = sourcingProgressCase()
	c.Offers[0].SentDate = ""
	if p := SourcingProgressFacts(&c, sourcingProjectID, sourcingPackageID, s, true); p.Steps[0].State != "in_progress" {
		t.Fatal("unsent invitation earned completion", p)
	}
	c, s = sourcingProgressCase()
	c.Decision.Source.DemandVersion++
	if p := SourcingProgressFacts(&c, sourcingProjectID, sourcingPackageID, s, true); p.Current || p.Steps[1].State != "blocked" {
		t.Fatal("old selection earned progress", p)
	}
	if p := SourcingProgressFacts(nil, sourcingProjectID, sourcingPackageID, s, true); len(p.Steps) != 3 || p.Current || p.Snapshot != nil {
		t.Fatal(p)
	}
}
