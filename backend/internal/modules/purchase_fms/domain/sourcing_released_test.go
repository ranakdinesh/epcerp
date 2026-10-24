package domain

import (
	"testing"
)

func TestSourcingOnlyImmutableReleasedCommercialEvidenceBecomesDownstreamAuthority(t *testing.T) {
	c := SourcingCase{Active: true, State: "approved"}
	if r, e := SourcingReleasedFact(c); e != nil || r != nil {
		t.Fatal("pending approval became downstream authority")
	}
	c.State = "released"
	if _, e := SourcingReleasedFact(c); e == nil {
		t.Fatal("release without captured evidence accepted")
	}
	_, _, source, _, _, now := sourcingSetup()
	offer := SourcingOffer{SupplierID: sourcingSupplier, UnitPrice: "10", Discount: "0", Tax: "0", Freight: "0", Other: "0"}
	c = SourcingCase{Active: true, State: "released", ID: sourcingCaseID, ProjectID: sourcingProjectID, PackageID: sourcingPackageID, Version: 7, ApproverID: sourcingApprover, ApprovedBy: sourcingApprover, ApprovedAt: &now, ProposedBy: sourcingOperator, ReleasedAt: &now, LOIReference: "LOI-actual", Decision: &SourcingDecision{Source: source, Offer: offer, FinancialReference: "approved:fingerprint"}}
	r, e := SourcingReleasedFact(c)
	if e != nil || r.Total != "20.05" || r.FinancialReference != "approved:fingerprint" || r.Source.Quantity != "2.0050" {
		t.Fatal(r, e)
	}
	r.Offer.UnitPrice = "999"
	r.Source.Blockers = append(r.Source.Blockers, "changed")
	if c.Decision.Offer.UnitPrice != "10" || len(c.Decision.Source.Blockers) != 0 {
		t.Fatal("downstream fact mutated source")
	}
}
