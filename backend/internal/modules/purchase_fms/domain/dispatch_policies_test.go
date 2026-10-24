package domain

import (
	"strings"
	"testing"
	"time"
)

const dispatchProject = "11111111-1111-4111-8111-111111111111"
const dispatchPkg = "22222222-2222-4222-8222-222222222222"
const dispatchActorID = "33333333-3333-4333-8333-333333333333"
const dispatchIdentity = "44444444-4444-4444-8444-444444444444"

func dispatchSource() DispatchSource {
	return DispatchSource{Available: true, Blockers: []string{}, DispatchSourceFacts: DispatchSourceFacts{ClearanceID: dispatchIdentity, ClearanceVersion: 6, ClearanceRevision: 3, Reference: "MDCC-1", ClearanceDate: "2026-10-10", ProjectID: dispatchProject, PackageID: dispatchPkg, QualityID: dispatchIdentity, QualityVersion: 23, QualityRevision: 2, ReportID: dispatchIdentity, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: dispatchIdentity, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: dispatchIdentity, InspectionVersion: 8}}
}
func dispatchOwner() Principal { return Principal{ID: dispatchActorID, Active: true} }
func dispatchCaps() DispatchAccess {
	return DispatchAccess{Read: true, Write: true, Scoped: true, Work: true}
}
func dispatchNow() DispatchTimestamp { return time.Date(2026, 10, 11, 5, 0, 0, 0, time.UTC) }
func dispatchClear() DispatchInput {
	return DispatchInput{Status: "dispatched", LRNumber: "LR-actual", Transporter: "Actual transporter", Date: "2026-10-11", DriverContact: "Client dispatch instructions", DocumentReference: "Actual MDCC reference"}
}
func TestDispatchMaterialDispatchActualEvidenceAndRevision(t *testing.T) {
	s := dispatchSource()
	in := dispatchClear()
	q := DispatchCommand{Action: "record", Reason: "Record actual MDCC", Dispatch: &in}
	r, e := DispatchApply(nil, dispatchOwner(), dispatchCaps(), q, s, dispatchNow(), dispatchIdentity)
	if e != nil || r.Dispatch.LRNumber != "LR-actual" || r.Source.Quantity != "1.000001" || r.Version != 1 {
		t.Fatal(e)
	}
	f, e := DispatchedFact(&r, s)
	if e != nil || f == nil {
		t.Fatal(e)
	}
	f.Source.Blockers = append(f.Source.Blockers, "Caller")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("mutable fact alias")
	}
	q.Version = 1
	if _, e = DispatchApply(&r, dispatchOwner(), dispatchCaps(), q, s, dispatchNow(), dispatchIdentity); e == nil {
		t.Fatal("cleared evidence overwritten")
	}
	next, e := DispatchApply(&r, dispatchOwner(), dispatchCaps(), DispatchCommand{Version: 1, Action: "revise", Reason: "Correct actual client MDCC"}, s, dispatchNow(), dispatchIdentity)
	if e != nil || next.Revision != 2 || next.Dispatch.Status != "pending" || next.Dispatch.LRNumber != "" || r.Dispatch.LRNumber != "LR-actual" {
		t.Fatal("revision reset/retention", e)
	}
	if _, e = DispatchedFact(&next, s); e == nil {
		t.Fatal("pending fact cleared")
	}
	changed := s
	changed.ClearanceVersion++
	if _, e = DispatchedFact(&r, changed); e != DispatchStale {
		t.Fatal("old quality approved dispatch", e)
	}
	if _, e = DispatchApply(&r, dispatchOwner(), dispatchCaps(), DispatchCommand{Version: 1, Action: "revise", Reason: "Review current quality"}, changed, dispatchNow(), dispatchIdentity); e != nil {
		t.Fatal(e)
	}
	s.Available = false
	if _, e = DispatchedFact(&r, s); e == nil {
		t.Fatal("held source cleared")
	}
}
func TestDispatchMaterialDispatchMissingIdentityDateBoundsAndCommands(t *testing.T) {
	for _, mutate := range []func(*DispatchInput){func(c *DispatchInput) { c.LRNumber = " " }, func(c *DispatchInput) { c.Date = "" }, func(c *DispatchInput) { c.Date = "2026-02-30" }, func(c *DispatchInput) { c.Date = "2026-10-09" }, func(c *DispatchInput) { c.Date = "2026-10-12" }, func(c *DispatchInput) { c.Transporter = strings.Repeat("x", 301) }, func(c *DispatchInput) { c.DriverContact = strings.Repeat("x", 101) }, func(c *DispatchInput) { c.Status = "approved" }} {
		c := dispatchClear()
		mutate(&c)
		if _, e := DispatchApply(nil, dispatchOwner(), dispatchCaps(), DispatchCommand{Action: "record", Reason: "Actual MDCC", Dispatch: &c}, dispatchSource(), dispatchNow(), dispatchIdentity); e == nil {
			t.Fatal("invalid client evidence passed", c)
		}
	}
	pending := DispatchInput{Status: "pending"}
	r, e := DispatchApply(nil, dispatchOwner(), dispatchCaps(), DispatchCommand{Action: "record", Reason: "Prepare", Dispatch: &pending}, dispatchSource(), dispatchNow(), dispatchIdentity)
	if e != nil {
		t.Fatal(e)
	}
	if e = DispatchCan(&r, dispatchOwner(), dispatchCaps(), "revise", dispatchSource()); e == nil {
		t.Fatal("unnecessary pending revision")
	}
	c := dispatchCaps()
	c.Work = false
	if _, e = DispatchApply(nil, dispatchOwner(), c, DispatchCommand{Action: "record", Reason: "x", Dispatch: &pending}, dispatchSource(), dispatchNow(), dispatchIdentity); e != DispatchForbidden {
		t.Fatal("unassigned write", e)
	}
	for _, q := range []DispatchCommand{{Action: "record", Reason: "x"}, {Action: "revise", Reason: "x", Dispatch: &pending}, {Action: "record", Reason: "", Dispatch: &pending}, {Version: -1, Action: "record", Reason: "x", Dispatch: &pending}} {
		if _, e := DispatchValidate(q); e == nil {
			t.Fatal("invalid command passed")
		}
	}
}

func TestDispatchDispatchOptionalEvidenceDoesNotInventEwayExpiryRule(t *testing.T) {
	c := dispatchClear()
	c.EwayValidUntil = "2000-01-01"
	if _, e := DispatchApply(nil, dispatchOwner(), dispatchCaps(), DispatchCommand{Action: "record", Reason: "Actual evidence", Dispatch: &c}, dispatchSource(), dispatchNow(), dispatchIdentity); e != nil {
		t.Fatal("invented e-way expiry restriction", e)
	}
	c.EwayValidUntil = "2027-01-01"
	c.LRDate = "2026-10-12"
	if _, e := DispatchApply(nil, dispatchOwner(), dispatchCaps(), DispatchCommand{Action: "record", Reason: "Actual evidence", Dispatch: &c}, dispatchSource(), dispatchNow(), dispatchIdentity); e == nil {
		t.Fatal("future actual LR date")
	}
	c.LRDate = "2026-10-11"
	c.InvoiceDate = "2026-02-30"
	if _, e := DispatchApply(nil, dispatchOwner(), dispatchCaps(), DispatchCommand{Action: "record", Reason: "Actual evidence", Dispatch: &c}, dispatchSource(), dispatchNow(), dispatchIdentity); e == nil {
		t.Fatal("invalid invoice date")
	}
}
