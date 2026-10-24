package domain

import (
	"strings"
	"testing"
	"time"
)

const gRNsProject = "11111111-1111-4111-8111-111111111111"
const gRNsPkg = "22222222-2222-4222-8222-222222222222"
const gRNsActorID = "33333333-3333-4333-8333-333333333333"
const gRNsIdentity = "44444444-4444-4444-8444-444444444444"

func gRNsSource() GRNsSource {
	return GRNsSource{Available: true, Blockers: []string{}, GRNsSourceFacts: GRNsSourceFacts{ReceiptID: gRNsIdentity, ReceiptVersion: 6, ReceiptRevision: 3, ArrivalDate: "2026-10-10", ReceivingOfficer: "Actual officer", PackagesCondition: "quarantine", DispatchID: gRNsIdentity, DispatchVersion: 6, DispatchRevision: 3, DispatchDate: "2026-10-10", LRNumber: "LR-1", Transporter: "Carrier", ClearanceID: gRNsIdentity, ClearanceVersion: 6, ClearanceRevision: 3, Reference: "MDCC-1", ClearanceDate: "2026-10-10", ProjectID: gRNsProject, PackageID: gRNsPkg, QualityID: gRNsIdentity, QualityVersion: 23, QualityRevision: 2, ReportID: gRNsIdentity, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: gRNsIdentity, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: gRNsIdentity, InspectionVersion: 8}}
}
func gRNsOwner() Principal   { return Principal{ID: gRNsActorID, Active: true} }
func gRNsCaps() GRNsAccess   { return GRNsAccess{Read: true, Write: true, Scoped: true, Work: true} }
func gRNsNow() GRNsTimestamp { return time.Date(2026, 10, 11, 5, 0, 0, 0, time.UTC) }
func gRNsIssued() GRNsGRNInput {
	return GRNsGRNInput{Status: "issued", Number: "GRN-actual", Date: "2026-10-11", Received: "1.000001", Accepted: "1", Rejected: "0.000001", WarrantyReference: "Actual warranty", WarrantyValidUntil: "2000-01-01", StoreKeeper: "Actual store keeper", DocumentReference: "Actual GRN reference"}
}
func TestGRNsGRNExactBreakdownSourceAndRevision(t *testing.T) {
	s := gRNsSource()
	in := gRNsIssued()
	q := GRNsCommand{Action: "record", Reason: "Actual inwarding", GRN: &in}
	r, e := GRNsApply(nil, gRNsOwner(), gRNsCaps(), q, s, gRNsNow(), gRNsIdentity)
	if e != nil || r.GRN.Received != "1.000001" || r.GRN.Accepted != "1" || r.GRN.Rejected != "0.000001" || r.GRN.UOM != "metre" || r.GRN.WarrantyValidUntil != "2000-01-01" {
		t.Fatal(e, r)
	}
	f, e := GRNsIssuedFact(&r, s)
	if e != nil || f == nil {
		t.Fatal(e)
	}
	f.Source.Blockers = append(f.Source.Blockers, "Caller")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("mutable fact alias")
	}
	q.Version = 1
	if _, e = GRNsApply(&r, gRNsOwner(), gRNsCaps(), q, s, gRNsNow(), gRNsIdentity); e != GRNsStale {
		t.Fatal("issued overwritten", e)
	}
	changed := s
	changed.ReceiptVersion++
	if _, e = GRNsIssuedFact(&r, changed); e != GRNsStale {
		t.Fatal("obsolete receipt", e)
	}
	next, e := GRNsApply(&r, gRNsOwner(), gRNsCaps(), GRNsCommand{Version: 1, Action: "revise", Reason: "Review actual receiving correction"}, changed, gRNsNow(), gRNsIdentity)
	if e != nil || next.GRN.Status != "pending" || next.GRN.Received != "0" || next.GRN.Accepted != "0" || next.GRN.Rejected != "0" || next.GRN.Number != "" || next.GRN.WarrantyReference != "" || next.GRN.UOM != "metre" || next.Revision != 2 || r.GRN.Number == "" {
		t.Fatal("revision reset/retention", e)
	}
	if _, e = GRNsIssuedFact(&next, changed); e == nil {
		t.Fatal("pending issued")
	}
	s.Available = false
	if _, e = GRNsIssuedFact(&r, s); e == nil {
		t.Fatal("held issued")
	}
}
func TestGRNsGRNQuantityAndActualDateRules(t *testing.T) {
	for _, mutate := range []func(*GRNsGRNInput){
		func(r *GRNsGRNInput) { r.Number = " " }, func(r *GRNsGRNInput) { r.Date = "" }, func(r *GRNsGRNInput) { r.Date = "2026-02-30" }, func(r *GRNsGRNInput) { r.Date = "2026-10-09" }, func(r *GRNsGRNInput) { r.Date = "2026-10-12" },
		func(r *GRNsGRNInput) { r.Received = "0" }, func(r *GRNsGRNInput) { r.Received = "1.000002" }, func(r *GRNsGRNInput) { r.Accepted = "1.000001" }, func(r *GRNsGRNInput) { r.Accepted = "-1" }, func(r *GRNsGRNInput) { r.Rejected = "1e-6" }, func(r *GRNsGRNInput) { r.Received = "1.0000001" }, func(r *GRNsGRNInput) { r.Received = "1000000000000" },
		func(r *GRNsGRNInput) { r.Status = "accepted" }, func(r *GRNsGRNInput) { r.WarrantyValidUntil = "2026-02-30" }, func(r *GRNsGRNInput) { r.StoreKeeper = strings.Repeat("x", 301) },
	} {
		r := gRNsIssued()
		mutate(&r)
		if _, e := GRNsApply(nil, gRNsOwner(), gRNsCaps(), GRNsCommand{Action: "record", Reason: "Actual", GRN: &r}, gRNsSource(), gRNsNow(), gRNsIdentity); e == nil {
			t.Fatal("invalid GRN passed", r)
		}
	}
	// Preserve Python's partial classification and zero accepted stock; optional warranty never becomes a gate.
	r := gRNsIssued()
	r.Accepted = "0"
	r.Rejected = "0"
	r.StoreKeeper = ""
	r.WarrantyReference = ""
	r.WarrantyValidUntil = ""
	r.DocumentReference = ""
	if _, e := GRNsApply(nil, gRNsOwner(), gRNsCaps(), GRNsCommand{Action: "record", Reason: "Actual unclassified inwarding", GRN: &r}, gRNsSource(), gRNsNow(), gRNsIdentity); e != nil {
		t.Fatal("invented complete classification/warranty restriction", e)
	}
	pending := GRNsGRNInput{Status: "pending"}
	out, e := GRNsApply(nil, gRNsOwner(), gRNsCaps(), GRNsCommand{Action: "record", Reason: "Prepare", GRN: &pending}, gRNsSource(), gRNsNow(), gRNsIdentity)
	if e != nil || out.GRN.Received != "0" || out.GRN.Accepted != "0" || out.GRN.UOM != "metre" {
		t.Fatal("invented draft quantities", e)
	}
	if GRNsCan(&out, gRNsOwner(), gRNsCaps(), "revise", gRNsSource()) == nil {
		t.Fatal("unnecessary draft revision")
	}
	c := gRNsCaps()
	c.Work = false
	if _, e = GRNsApply(nil, gRNsOwner(), c, GRNsCommand{Action: "record", Reason: "Actual", GRN: &r}, gRNsSource(), gRNsNow(), gRNsIdentity); e != GRNsForbidden {
		t.Fatal("unassigned write", e)
	}
}
