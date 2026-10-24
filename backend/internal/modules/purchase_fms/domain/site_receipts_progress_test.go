package domain

import (
	"testing"
)

func TestSiteReceiptsProgressPreservesActualCompletionWithoutGrantingExecution(t *testing.T) {
	s := siteReceiptsSource()
	input := siteReceiptsDelivered()
	r, err := SiteReceiptsApply(nil, siteReceiptsOwner(), siteReceiptsCaps(), SiteReceiptsCommand{Action: "record", Reason: "Actual recorded completion", Receipt: &input}, s, siteReceiptsNow(), siteReceiptsIdentity)
	if err != nil {
		t.Fatal(err)
	}
	p := SiteReceiptsProgressFact(&r, s.SiteReceiptsSourceFacts, true)
	if p.State != "completed" || !p.Current || p.Snapshot == nil {
		t.Fatal(p)
	}
	p.Snapshot.Source.Blockers = append(p.Snapshot.Source.Blockers, "changed")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("aliased reporting snapshot")
	}
	s.Available = false
	s.Blockers = []string{"Held project"}
	if SiteReceiptsProgressFact(&r, s.SiteReceiptsSourceFacts, true).State != "completed" {
		t.Fatal("hold erased recorded completion")
	}
	if _, err := SiteReceiptsDeliveredFact(&r, s); err == nil {
		t.Fatal("reporting removed execution hold")
	}
	changed := s.SiteReceiptsSourceFacts
	changed.DispatchVersion++
	if p := SiteReceiptsProgressFact(&r, changed, true); p.State != "blocked" || p.Current {
		t.Fatal("changed upstream earned completion", p)
	}
	if p := SiteReceiptsProgressFact(&r, s.SiteReceiptsSourceFacts, false); p.State != "blocked" || p.Current {
		t.Fatal("missing upstream proof", p)
	}
	r.Receipt.ArrivalDate = ""
	if p := SiteReceiptsProgressFact(&r, s.SiteReceiptsSourceFacts, true); p.State != "blocked" || p.Snapshot != nil {
		t.Fatal("incomplete actual evidence earned completion", p)
	}
	r.Receipt.Status = "pending"
	if p := SiteReceiptsProgressFact(&r, s.SiteReceiptsSourceFacts, true); p.State != "not_started" || p.Snapshot != nil {
		t.Fatal("pending record earned completion", p)
	}
}
