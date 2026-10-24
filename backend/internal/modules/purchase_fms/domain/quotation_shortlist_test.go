package domain

import "testing"

func TestShortlistTwoDesignationsSnapshotChoiceAndRedaction(t *testing.T) {
	s := quotationApprovalsSourceFixture()
	s.Quotes[0].Total = "145.69"
	alt := s.Quotes[0]
	alt.Key = quotationApprovalsDocID + ":2"
	alt.Revision = 2
	alt.Total = "146.00"
	alt.Vendor = "Preferred supplier"
	alt.Lines = append([]QuotationApprovalsQuoteLine{}, alt.Lines...)
	alt.Lines[0].Quoted = "146.00"
	s.Quotes = append(s.Quotes, alt)
	nominations := []QuotationApprovalsNomination{{QuoteKey: s.Quotes[0].Key, BestPrice: true, BestPriceComment: "Lowest landed cost"}, {QuoteKey: alt.Key, Preferred: true, PreferredComment: "Earlier delivery"}}
	a, e := QuotationApprovalsShortlist(nominations, s)
	if e != nil || len(a.Proposals) != 2 {
		t.Fatal(a, e)
	}
	a.ID = quotationApprovalsDocID
	a.SubmittedBy = "executive"
	a.Reason = "Compare proposals"
	a.State = "pending_accounts"
	chosen, e := QuotationApprovalsSelectProposal(a, alt.Key)
	if e != nil || chosen.Total != "146.00" || chosen.ID != a.ID || len(chosen.Proposals) != 2 {
		t.Fatal(chosen, e)
	}
	redacted := QuotationApprovalsRedact([]QuotationApprovalsAttempt{chosen})[0]
	if redacted.Proposals[0].Snapshot.Lines[0].Budget != nil || chosen.Proposals[0].Snapshot.Lines[0].Budget == nil {
		t.Fatal("nested Finance budget leaked or original mutated")
	}
	nominations[0].QuoteKey = alt.Key
	nominations[1].QuoteKey = s.Quotes[0].Key
	if _, e = QuotationApprovalsShortlist(nominations, s); e == nil {
		t.Fatal("higher priced offer marked best")
	}
	if _, e = QuotationApprovalsSelectProposal(a, quotationApprovalsDocID+":3"); e == nil {
		t.Fatal("unsaved Finance proposal selected")
	}
}
func TestShortlistSameOfferBothAndValidation(t *testing.T) {
	s := quotationApprovalsSourceFixture()
	q := QuotationApprovalsNomination{QuoteKey: s.Quotes[0].Key, BestPrice: true, Preferred: true, BestPriceComment: "Lowest cost", PreferredComment: "Suitable terms"}
	if a, e := QuotationApprovalsShortlist([]QuotationApprovalsNomination{q}, s); e != nil || len(a.Proposals) != 1 {
		t.Fatal(e)
	}
	for _, change := range []func(*QuotationApprovalsNomination){func(n *QuotationApprovalsNomination) { n.BestPrice = false }, func(n *QuotationApprovalsNomination) { n.PreferredComment = "" }, func(n *QuotationApprovalsNomination) { n.BestPriceComment = " " }} {
		n := q
		change(&n)
		if _, e := QuotationApprovalsValidateNominations([]QuotationApprovalsNomination{n}); e == nil {
			t.Fatal("invalid nomination")
		}
	}
	if _, e := QuotationApprovalsValidateNominations([]QuotationApprovalsNomination{q, q}); e == nil {
		t.Fatal("duplicate source")
	}
}

func TestMixedGroupsRetainApprovedProposalAndBlockAllGroupGate(t *testing.T) {
	s := quotationApprovalsSourceFixture()
	a, e := QuotationApprovalsSnapshot(s.Quotes[0], s)
	if e != nil {
		t.Fatal(e)
	}
	a.State = "approved"
	second := s.Quotes[0]
	second.GroupID = "returned-group"
	second.Key = quotationApprovalsDocID + ":2"
	s.Quotes = append(s.Quotes, second)
	s.Groups = append(s.Groups, QuotationApprovalsGroup{ID: second.GroupID, ScopeRevision: 1})
	b, e := QuotationApprovalsSnapshot(second, s)
	if e != nil {
		t.Fatal(e)
	}
	b.State = "rejected"
	b.ReturnAction = "change_vendor"
	_, latest, ready := QuotationApprovalsEvaluate([]QuotationApprovalsAttempt{a, b}, s)
	if ready || latest[a.GroupID].EffectiveState != "approved" || latest[b.GroupID].EffectiveState != "rejected" || latest[b.GroupID].ReturnAction != "change_vendor" {
		t.Fatal(latest, ready)
	}
}
