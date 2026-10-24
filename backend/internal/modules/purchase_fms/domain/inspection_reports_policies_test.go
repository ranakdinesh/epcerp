package domain

import (
	"strings"
	"testing"
	"time"
)

const inspectionReportsProject = "11111111-1111-4111-8111-111111111111"
const inspectionReportsPkg = "22222222-2222-4222-8222-222222222222"
const inspectionReportsActorID = "33333333-3333-4333-8333-333333333333"
const inspectionReportsIdentity = "44444444-4444-4444-8444-444444444444"

func inspectionReportsSource() InspectionReportsSource {
	return InspectionReportsSource{Available: true, Blockers: []string{}, InspectionReportsSourceFacts: InspectionReportsSourceFacts{ProjectID: inspectionReportsProject, PackageID: inspectionReportsPkg, InspectionID: inspectionReportsIdentity, InspectionVersion: 6, InspectionRevision: 3, ActualDate: "2026-10-10", OrderID: inspectionReportsIdentity, OrderVersion: 2, Number: "Actual PO", SupplierName: "Actual supplier", Description: "Inspected material", Quantity: "10.000001", UOM: "metre", QAPReference: "Actual approved QAP"}}
}
func inspectionReportsOwner() Principal { return Principal{ID: inspectionReportsActorID, Active: true} }
func inspectionReportsCaps() InspectionReportsAccess {
	return InspectionReportsAccess{Read: true, Write: true, Scoped: true, Work: true}
}
func inspectionReportsNow() InspectionReportsTimestamp {
	return time.Date(2026, 10, 11, 5, 0, 0, 0, time.UTC)
}
func inspectionReportsIssued() InspectionReportsReportInput {
	return InspectionReportsReportInput{Status: "issued", IRNumber: "Actual IR", IRNNumber: "Actual IRN", IssueDate: "2026-10-11", Authority: "Actual inspector", Quantity: "5.000001", DocumentReference: "Actual signed reference"}
}
func TestInspectionReportsReportExactQuantityAndTruthfulIssuance(t *testing.T) {
	s := inspectionReportsSource()
	q := InspectionReportsCommand{Action: "record", Reason: "Actual report issuance"}
	in := inspectionReportsIssued()
	q.Report = &in
	r, e := InspectionReportsApply(nil, inspectionReportsOwner(), inspectionReportsCaps(), q, s, inspectionReportsNow(), inspectionReportsIdentity)
	if e != nil || r.Report.Quantity != "5.000001" || r.Report.UOM != "metre" || r.Version != 1 || r.Revision != 1 {
		t.Fatal(e)
	}
	f, e := InspectionReportsIssuedFact(&r, s)
	if e != nil || f == nil {
		t.Fatal(e)
	}
	f.Source.Blockers = append(f.Source.Blockers, "Caller change")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("mutable fact alias")
	}
	q.Version = 1
	if _, e = InspectionReportsApply(&r, inspectionReportsOwner(), inspectionReportsCaps(), q, s, inspectionReportsNow(), inspectionReportsIdentity); e == nil {
		t.Fatal("issued report overwritten")
	}
	q = InspectionReportsCommand{Version: 1, Action: "revise", Reason: "Actual corrected report"}
	next, e := InspectionReportsApply(&r, inspectionReportsOwner(), inspectionReportsCaps(), q, s, inspectionReportsNow(), inspectionReportsIdentity)
	if e != nil || next.Revision != 2 || next.Report.Status != "pending" || next.Report.IRNNumber != "" || next.Report.Quantity != "0" || r.Report.Status != "issued" {
		t.Fatal("revision lost evidence", e)
	}
	if _, e = InspectionReportsIssuedFact(&next, s); e == nil {
		t.Fatal("revision remained issued")
	}
	changed := s
	changed.InspectionVersion++
	if _, e = InspectionReportsIssuedFact(&r, changed); e != InspectionReportsStale {
		t.Fatal("changed inspection reused", e)
	}
}
func TestInspectionReportsReportIssuanceRejectsMissingAndInvalidFacts(t *testing.T) {
	cases := map[string]func(*InspectionReportsReportInput){"status": func(q *InspectionReportsReportInput) { q.Status = "approved" }, "IR": func(q *InspectionReportsReportInput) { q.IRNumber = "" }, "IRN": func(q *InspectionReportsReportInput) { q.IRNNumber = "" }, "date missing": func(q *InspectionReportsReportInput) { q.IssueDate = "" }, "date future": func(q *InspectionReportsReportInput) { q.IssueDate = "2026-10-12" }, "date before visit": func(q *InspectionReportsReportInput) { q.IssueDate = "2026-10-09" }, "invalid calendar": func(q *InspectionReportsReportInput) { q.IssueDate = "2026-02-30" }, "quantity zero": func(q *InspectionReportsReportInput) { q.Quantity = "0" }, "over by micro unit": func(q *InspectionReportsReportInput) { q.Quantity = "10.000002" }, "negative": func(q *InspectionReportsReportInput) { q.Quantity = "-1" }, "exponent": func(q *InspectionReportsReportInput) { q.Quantity = "1e1" }, "seventh decimal": func(q *InspectionReportsReportInput) { q.Quantity = "1.0000001" }, "leading zero": func(q *InspectionReportsReportInput) { q.Quantity = "01" }, "large": func(q *InspectionReportsReportInput) { q.Quantity = "1000000000000" }, "authority too long": func(q *InspectionReportsReportInput) { q.Authority = strings.Repeat("x", 301) }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			q := inspectionReportsIssued()
			change(&q)
			if _, e := InspectionReportsApply(nil, inspectionReportsOwner(), inspectionReportsCaps(), InspectionReportsCommand{Action: "record", Reason: "Actual issue", Report: &q}, inspectionReportsSource(), inspectionReportsNow(), inspectionReportsIdentity); e == nil {
				t.Fatal("invalid report allowed")
			}
		})
	}
	for _, qty := range []string{"0.000001", "10.000001"} {
		q := inspectionReportsIssued()
		q.Quantity = qty
		if _, e := InspectionReportsApply(nil, inspectionReportsOwner(), inspectionReportsCaps(), InspectionReportsCommand{Action: "record", Reason: "Actual issue", Report: &q}, inspectionReportsSource(), inspectionReportsNow(), inspectionReportsIdentity); e != nil {
			t.Fatal("exact quantity failed", qty, e)
		}
	}
}
func TestInspectionReportsReportPendingPermissionsSourceAndVersion(t *testing.T) {
	s := inspectionReportsSource()
	q := InspectionReportsCommand{Action: "record", Reason: "Prepare actual report", Report: &InspectionReportsReportInput{Status: "pending"}}
	r, e := InspectionReportsApply(nil, inspectionReportsOwner(), inspectionReportsCaps(), q, s, inspectionReportsNow(), inspectionReportsIdentity)
	if e != nil || r.Report.Quantity != "0" {
		t.Fatal(e)
	}
	q.Version = 1
	q.Report.IRNumber = "Draft IR"
	if _, e = InspectionReportsApply(&r, inspectionReportsOwner(), inspectionReportsCaps(), q, s, inspectionReportsNow(), inspectionReportsIdentity); e != nil {
		t.Fatal(e)
	}
	q.Version = 0
	if _, e = InspectionReportsApply(&r, inspectionReportsOwner(), inspectionReportsCaps(), q, s, inspectionReportsNow(), inspectionReportsIdentity); e != InspectionReportsStale {
		t.Fatal(e)
	}
	for _, c := range []InspectionReportsAccess{{Write: true, Work: true}, {Read: true, Work: true}, {Read: true, Write: true}} {
		if _, e = InspectionReportsApply(nil, inspectionReportsOwner(), c, q, s, inspectionReportsNow(), inspectionReportsIdentity); e != InspectionReportsForbidden {
			t.Fatal("scope bypass", e)
		}
	}
	s.Available = false
	if _, e = InspectionReportsApply(nil, inspectionReportsOwner(), inspectionReportsCaps(), q, s, inspectionReportsNow(), inspectionReportsIdentity); e == nil {
		t.Fatal("held source bypass")
	}
}
