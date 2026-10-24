package domain

import (
	"testing"
)

const quotationApprovalsLineID = "11111111-1111-4111-8111-111111111111"
const quotationApprovalsDocID = "22222222-2222-4222-8222-222222222222"

func quotationApprovalsSourceFixture() QuotationApprovalsSource {
	return QuotationApprovalsSource{ProjectID: "project", BOID: "bo", Currency: "INR", Available: true, Groups: []QuotationApprovalsGroup{{ID: "group", ScopeRevision: 1}}, Budget: &QuotationApprovalsBudget{Revision: 1, Currency: "INR", Allocations: []QuotationApprovalsBudgetAllocation{{BOID: "bo", LineID: quotationApprovalsLineID, Amount: "200.00"}}}, Verifications: map[string]QuotationApprovalsVerification{"vendor": {State: "verified", Fingerprint: "current", Message: "verified"}}, Quotes: []QuotationApprovalsQuote{{Key: quotationApprovalsDocID + ":1", DocumentID: quotationApprovalsDocID, Revision: 1, GroupID: "group", ScopeRevision: 1, VendorID: "vendor", Total: "145.69", Eligible: true, Lines: []QuotationApprovalsQuoteLine{{LineID: quotationApprovalsLineID, Quantity: "10.0000", Quoted: "145.69", Item: QuotationApprovalsItem{SKU: "CAB", Description: "Cable"}}}}}}
}
func TestQuotationApprovalsEvidenceErrorIdentifiesTheSupplier(t *testing.T) {
	for _, tc := range []struct{ vendor, reason, want string }{
		{"Polycab Wires", "Required evidence missing: cheque", "Supplier Polycab Wires: Required evidence missing: cheque"},
		{"Havells", "Required evidence missing: pan, cheque", "Supplier Havells: Required evidence missing: pan, cheque"},
		{"", "", "Supplier vendor: Supplier documents require Finance verification"},
	} {
		s := quotationApprovalsSourceFixture()
		s.Quotes[0].Vendor = tc.vendor
		s.Verifications["vendor"] = QuotationApprovalsVerification{State: "blocked", Message: tc.reason}
		_, err := QuotationApprovalsSnapshot(s.Quotes[0], s)
		failure, ok := err.(*QuotationApprovalsError)
		if !ok || failure.Status != 422 || failure.Field != "vendor" || failure.Message != tc.want {
			t.Fatalf("supplier-specific evidence error: %v", err)
		}
	}
}
func TestQuotationApprovalsSnapshotBudgetPreviewAndSourceGates(t *testing.T) {
	s := quotationApprovalsSourceFixture()
	a, e := QuotationApprovalsSnapshot(s.Quotes[0], s)
	if e != nil || a.Currency != "INR" || a.BudgetRevision != 1 || *a.Lines[0].Budget != "200.00" || *a.Lines[0].Difference != "-54.31" {
		t.Fatal("snapshot", e)
	}
	p, e := QuotationApprovalsBudgetPreview(s.Quotes[0], s)
	if e != nil || p.BudgetRevision == nil || *p.Lines[0].Difference != "-54.31" {
		t.Fatal("preview", e)
	}
	s.Quotes[0].Lines[0].Quoted = "200.01"
	p, e = QuotationApprovalsBudgetPreview(s.Quotes[0], s)
	if e != nil || *p.Lines[0].Difference != "+0.01" {
		t.Fatal("signed exact difference", e)
	}
	for _, change := range []func(*QuotationApprovalsSource){func(s *QuotationApprovalsSource) {
		s.Verifications["vendor"] = QuotationApprovalsVerification{State: "stale", Message: "Changed evidence"}
	}, func(s *QuotationApprovalsSource) { s.Quotes[0].Eligible = false }} {
		s := quotationApprovalsSourceFixture()
		change(&s)
		if _, e := QuotationApprovalsSnapshot(s.Quotes[0], s); e == nil {
			t.Fatal("invalid submission source accepted")
		}
	}
}
func TestQuotationApprovalsEveryItemDecisionAndEffectiveReadiness(t *testing.T) {
	s := quotationApprovalsSourceFixture()
	a, e := QuotationApprovalsSnapshot(s.Quotes[0], s)
	if e != nil {
		t.Fatal(e)
	}
	a.ID = "attempt"
	a.State = "pending_accounts"
	for _, d := range [][]QuotationApprovalsDecision{nil, {{LineID: quotationApprovalsDocID, Decision: "accept", Comment: "Reviewed"}}, {{LineID: quotationApprovalsLineID, Decision: "accept", Comment: "Reviewed"}, {LineID: quotationApprovalsLineID, Decision: "accept", Comment: "Duplicate"}}} {
		if _, e := QuotationApprovalsDecide(a, d); e == nil {
			t.Fatal("invalid item decisions accepted")
		}
	}
	approved, e := QuotationApprovalsDecide(a, []QuotationApprovalsDecision{{LineID: quotationApprovalsLineID, Decision: "accept", Comment: "Within budget"}})
	if e != nil || approved.State != "approved" || a.Lines[0].Decision != nil {
		t.Fatal("immutable review copy", e)
	}
	rows, _, ready := QuotationApprovalsEvaluate([]QuotationApprovalsAttempt{approved}, s)
	if !ready || rows[0].EffectiveState != "approved" {
		t.Fatal("all groups approved")
	}
	s.Groups = append(s.Groups, QuotationApprovalsGroup{ID: "unquoted"})
	if _, _, ready = QuotationApprovalsEvaluate([]QuotationApprovalsAttempt{approved}, s); ready {
		t.Fatal("unquoted group incorrectly ready")
	}
	s = quotationApprovalsSourceFixture()
	redacted := QuotationApprovalsRedact([]QuotationApprovalsAttempt{approved})
	if redacted[0].Lines[0].Budget != nil || redacted[0].Lines[0].Difference != nil || approved.Lines[0].Budget == nil {
		t.Fatal("financial fields leaked/mutated")
	}
	for _, change := range []func(*QuotationApprovalsSource){func(s *QuotationApprovalsSource) { s.Available = false }, func(s *QuotationApprovalsSource) { s.Quotes[0].Eligible = false }, func(s *QuotationApprovalsSource) { s.Quotes[0].ScopeRevision++ }, func(s *QuotationApprovalsSource) {
		s.Verifications["vendor"] = QuotationApprovalsVerification{State: "verified", Fingerprint: "replacement"}
	}} {
		s := quotationApprovalsSourceFixture()
		change(&s)
		rows, _, ready := QuotationApprovalsEvaluate([]QuotationApprovalsAttempt{approved}, s)
		if ready || rows[0].EffectiveState != "stale" || rows[0].Message == "" {
			t.Fatal("changed source remained approved")
		}
	}
	rejected, e := QuotationApprovalsDecide(a, []QuotationApprovalsDecision{{LineID: quotationApprovalsLineID, Decision: "reject", Comment: "Revise price"}})
	if e != nil || rejected.State != "rejected" {
		t.Fatal("rejection", e)
	}
	if _, _, ready := QuotationApprovalsEvaluate([]QuotationApprovalsAttempt{rejected}, quotationApprovalsSourceFixture()); ready {
		t.Fatal("rejected source ready")
	}
}

func TestQuotationGroupDecisionAllowsOptionalItemNotes(t *testing.T) {
	for _, outcome := range []string{"accept", "reject"} {
		source := quotationApprovalsSourceFixture()
		attempt, err := QuotationApprovalsSnapshot(source.Quotes[0], source)
		if err != nil {
			t.Fatal(err)
		}
		command := QuotationApprovalsCommand{Version: 1, AttemptID: quotationApprovalsDocID, Reason: "Finance reviewed the complete group", Decisions: []QuotationApprovalsDecision{{LineID: quotationApprovalsLineID, Decision: outcome, Comment: "  "}}}
		valid, err := command.Validate("review")
		if err != nil {
			t.Fatal("optional item notes rejected", err)
		}
		decided, err := QuotationApprovalsDecide(attempt, valid.Decisions)
		if err != nil || decided.Lines[0].Comment != "" {
			t.Fatal("blank notes were rejected or fabricated", decided, err)
		}
		command.Reason = ""
		if _, err := command.Validate("review"); err == nil {
			t.Fatal("overall reason became optional")
		}
	}
}
