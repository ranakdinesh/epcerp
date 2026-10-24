package domain

import (
	"testing"
	"time"
)

func quotationApprovalsProgressRecord(t *testing.T) (QuotationApprovalsRecord, QuotationApprovalsSource) {
	t.Helper()
	s := quotationApprovalsSourceFixture()
	a, e := QuotationApprovalsSnapshot(s.Quotes[0], s)
	if e != nil {
		t.Fatal(e)
	}
	a, e = QuotationApprovalsDecide(a, []QuotationApprovalsDecision{{LineID: quotationApprovalsLineID, Decision: "accept", Comment: "Actual review"}})
	if e != nil {
		t.Fatal(e)
	}
	stamp := time.Date(2026, 10, 11, 5, 0, 0, 0, time.UTC)
	actor := quotationApprovalsDocID
	decision := quotationApprovalsLineID
	a.SubmittedBy = quotationApprovalsLineID
	a.ReviewedBy = &actor
	a.ReviewedAt = &stamp
	a.DecisionID = &decision
	return QuotationApprovalsRecord{ProjectID: s.ProjectID, BOID: s.BOID, Active: true, Version: 1, Attempts: []QuotationApprovalsAttempt{a}}, s
}
func TestQuotationApprovalsApprovalProgressPreservesHoldButNotChangedProof(t *testing.T) {
	r, s := quotationApprovalsProgressRecord(t)
	p, e := QuotationApprovalsProgressFacts(&r, s)
	if e != nil || !p.Ready || !p.Started || len(p.ApprovedGroupIDs) != 1 {
		t.Fatal(p, e)
	}
	s.Available = false
	p, e = QuotationApprovalsProgressFacts(&r, s)
	if e != nil || !p.Ready {
		t.Fatal("hold erased approval", p, e)
	}
	if _, _, ready := QuotationApprovalsEvaluate(r.Attempts, s); ready {
		t.Fatal("reporting relaxed approval execution availability")
	}
	for _, edit := range []func(*QuotationApprovalsSource){func(s *QuotationApprovalsSource) {
		s.Verifications["vendor"] = QuotationApprovalsVerification{State: "verified", Fingerprint: "changed"}
	}, func(s *QuotationApprovalsSource) { s.Quotes[0].Eligible = false }, func(s *QuotationApprovalsSource) { s.Groups[0].ScopeRevision++ }} {
		r, s := quotationApprovalsProgressRecord(t)
		edit(&s)
		p, e := QuotationApprovalsProgressFacts(&r, s)
		if e != nil || p.Ready || len(p.ApprovedGroupIDs) != 0 {
			t.Fatal("changed proof earned approval", p, e)
		}
	}
	r, s = quotationApprovalsProgressRecord(t)
	r.Attempts[0].DecisionID = nil
	if p, e := QuotationApprovalsProgressFacts(&r, s); e != nil || p.Ready {
		t.Fatal("no actual Finance decision", p, e)
	}
}
func TestQuotationApprovalsApprovalProgressIncludesUnquotedGroupsAndRejectsForeignScope(t *testing.T) {
	r, s := quotationApprovalsProgressRecord(t)
	s.Groups = append(s.Groups, QuotationApprovalsGroup{ID: "unquoted", ScopeRevision: 1})
	p, e := QuotationApprovalsProgressFacts(&r, s)
	if e != nil || p.Ready || len(p.Groups) != 2 || len(p.ApprovedGroupIDs) != 1 {
		t.Fatal(p, e)
	}
	r.BOID = "foreign"
	if _, e = QuotationApprovalsProgressFacts(&r, s); e == nil {
		t.Fatal("foreign record counted")
	}
}

func TestQuotationApprovalProgressAllowsEmptyOptionalItemNotes(t *testing.T) {
	record, source := quotationApprovalsProgressRecord(t)
	record.Attempts[0].Lines[0].Comment = ""
	if progress, err := QuotationApprovalsProgressFacts(&record, source); err != nil || !progress.Ready {
		t.Fatal("optional notes blocked independent approval readiness", progress, err)
	}
}
