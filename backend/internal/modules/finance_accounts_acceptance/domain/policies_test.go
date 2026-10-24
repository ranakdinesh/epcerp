package domain

import (
	"strings"
	"testing"
	"time"
)

const project = "11111111-1111-4111-8111-111111111111"
const pkg = "22222222-2222-4222-8222-222222222222"
const actorID = "33333333-3333-4333-8333-333333333333"
const identity = "44444444-4444-4444-8444-444444444444"

func source() Source {
	return Source{Available: true, Blockers: []string{}, SourceFacts: SourceFacts{EngineerID: identity, EngineerVersion: 6, EngineerRevision: 3, ConfirmationDate: "2026-10-11", EngineerName: "Actual engineer", Currency: "INR", GRNID: identity, GRNVersion: 6, GRNRevision: 3, GRNNumber: "GRN-1", GRNDate: "2026-10-10", Received: "1.000001", Accepted: "1", Rejected: "0.000001", ReceiptID: identity, ReceiptVersion: 6, ReceiptRevision: 3, ArrivalDate: "2026-10-10", ReceivingOfficer: "Actual officer", PackagesCondition: "quarantine", DispatchID: identity, DispatchVersion: 6, DispatchRevision: 3, DispatchDate: "2026-10-10", LRNumber: "LR-1", Transporter: "Carrier", ClearanceID: identity, ClearanceVersion: 6, ClearanceRevision: 3, Reference: "MDCC-1", ClearanceDate: "2026-10-10", ProjectID: project, PackageID: pkg, QualityID: identity, QualityVersion: 23, QualityRevision: 2, ReportID: identity, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: identity, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: identity, InspectionVersion: 8}}
}
func owner() Actor   { return Actor{ID: actorID, Active: true} }
func caps() Access   { return Access{Read: true, Write: true, Scoped: true, Work: true} }
func now() Timestamp { return time.Date(2026, 10, 11, 5, 0, 0, 0, time.UTC) }

func accepted() AcceptanceInput {
	return AcceptanceInput{Status: "accepted", Date: "2026-10-11", OfficerName: "Actual Accounts officer", BookedAmount: "999999999999.99", VoucherReference: "Actual voucher", Notes: "Actual Accounts review"}
}
func TestAccountsCurrentSourceAndReasonedRevision(t *testing.T) {
	s := source()
	in := accepted()
	q := Command{Action: "record", Reason: "Finance accepts", Acceptance: &in}
	r, e := Apply(nil, owner(), caps(), q, s, now(), identity)
	if e != nil {
		t.Fatal(e)
	}
	f, e := AcceptedFact(&r, s)
	if e != nil || f == nil {
		t.Fatal(e)
	}
	f.Source.Blockers = append(f.Source.Blockers, "caller")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("mutable fact")
	}
	q.Version = 1
	if _, e = Apply(&r, owner(), caps(), q, s, now(), identity); e != Stale {
		t.Fatal("accepted overwritten", e)
	}
	changed := s
	changed.EngineerVersion++
	if _, e = AcceptedFact(&r, changed); e != Stale {
		t.Fatal("obsolete engineer", e)
	}
	next, e := Apply(&r, owner(), caps(), Command{Version: 1, Action: "revise", Reason: "Review corrected engineer evidence"}, changed, now(), identity)
	if e != nil || next.Revision != 2 || next.Acceptance.Status != "pending" || next.Acceptance.BookedAmount != "0" || next.Acceptance.OfficerName != "" || next.Acceptance.VoucherReference != "" || r.Acceptance.VoucherReference == "" {
		t.Fatal("revision reset/retention", e, next)
	}
	if _, e = AcceptedFact(&next, changed); e == nil {
		t.Fatal("pending accepted")
	}
	s.Available = false
	if _, e = AcceptedFact(&r, s); e == nil {
		t.Fatal("held accepted")
	}
	for _, event := range []Event{{After: r}, {Action: "revise", Before: &r, After: next}} {
		if !FinancialEvent(event) {
			t.Fatal("lost Finance decision")
		}
	}
	if EventAction(Event{After: r}) != "approved" || EventAction(Event{Action: "revise", Before: &r, After: next}) != "returned" || FinancialEvent(Event{After: next}) {
		t.Fatal("decision/notices classification")
	}
}
func TestAccountsRequiredDatesExactMoneyOptionalReferencesAndFinanceScope(t *testing.T) {
	for _, mutate := range []func(*AcceptanceInput){func(v *AcceptanceInput) { v.OfficerName = " " }, func(v *AcceptanceInput) { v.Date = "" }, func(v *AcceptanceInput) { v.Date = "2026-10-10" }, func(v *AcceptanceInput) { v.Date = "2026-10-12" }, func(v *AcceptanceInput) { v.Date = "2026-02-30" }, func(v *AcceptanceInput) { v.Status = "approved" }, func(v *AcceptanceInput) { v.OfficerName = strings.Repeat("x", 301) }, func(v *AcceptanceInput) { v.VoucherReference = strings.Repeat("x", 201) }, func(v *AcceptanceInput) { v.Notes = strings.Repeat("x", 4001) }} {
		in := accepted()
		mutate(&in)
		if _, e := Apply(nil, owner(), caps(), Command{Action: "record", Reason: "Actual", Acceptance: &in}, source(), now(), identity); e == nil {
			t.Fatal("invalid accepted", in)
		}
	}
	for _, money := range []string{"-1", "1.001", "1e3", "1000000000000", "NaN", "+1", "01"} {
		in := accepted()
		in.BookedAmount = money
		if _, e := Apply(nil, owner(), caps(), Command{Action: "record", Reason: "Actual", Acceptance: &in}, source(), now(), identity); e == nil {
			t.Fatal("inexact money", money)
		}
	}
	in := accepted()
	in.BookedAmount = "0"
	in.VoucherReference = ""
	in.Notes = ""
	s := source()
	s.Accepted = "0"
	s.Rejected = "0"
	if _, e := Apply(nil, owner(), caps(), Command{Action: "record", Reason: "Actual", Acceptance: &in}, s, now(), identity); e != nil {
		t.Fatal("invented voucher/positive booking/stock gate", e)
	}
	pending := AcceptanceInput{Status: "pending"}
	out, e := Apply(nil, owner(), caps(), Command{Action: "record", Reason: "Prepare", Acceptance: &pending}, source(), now(), identity)
	if e != nil || out.Acceptance.BookedAmount != "0" || out.Acceptance.OfficerName != "" {
		t.Fatal(e)
	}
	c := caps()
	c.Work = false
	if _, e = Apply(nil, owner(), c, Command{Action: "record", Reason: "Actual", Acceptance: &in}, source(), now(), identity); e != Forbidden {
		t.Fatal("no Finance authority", e)
	}
}
