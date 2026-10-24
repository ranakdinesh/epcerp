package domain

import (
	"strings"
	"testing"
	"time"
)

const clientClearanceProject = "11111111-1111-4111-8111-111111111111"
const clientClearancePkg = "22222222-2222-4222-8222-222222222222"
const clientClearanceActorID = "33333333-3333-4333-8333-333333333333"
const clientClearanceIdentity = "44444444-4444-4444-8444-444444444444"

func clientClearanceSource() ClientClearanceSource {
	return ClientClearanceSource{Available: true, Blockers: []string{}, ClientClearanceSourceFacts: ClientClearanceSourceFacts{ProjectID: clientClearanceProject, PackageID: clientClearancePkg, QualityID: clientClearanceIdentity, QualityVersion: 23, QualityRevision: 2, ReportID: clientClearanceIdentity, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: clientClearanceIdentity, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: clientClearanceIdentity, InspectionVersion: 8}}
}
func clientClearanceOwner() Principal { return Principal{ID: clientClearanceActorID, Active: true} }
func clientClearanceCaps() ClientClearanceAccess {
	return ClientClearanceAccess{Read: true, Write: true, Scoped: true, Work: true}
}
func clientClearanceNow() ClientClearanceTimestamp {
	return time.Date(2026, 10, 11, 5, 0, 0, 0, time.UTC)
}
func clientClearanceClear() ClientClearanceClearanceInput {
	return ClientClearanceClearanceInput{Status: "cleared", Reference: "MDCC-actual", Date: "2026-10-11", Remarks: "Client dispatch instructions", DocumentReference: "Actual MDCC reference"}
}
func TestClientClearanceClientClearanceActualEvidenceAndRevision(t *testing.T) {
	s := clientClearanceSource()
	in := clientClearanceClear()
	q := ClientClearanceCommand{Action: "record", Reason: "Record actual MDCC", Clearance: &in}
	r, e := ClientClearanceApply(nil, clientClearanceOwner(), clientClearanceCaps(), q, s, clientClearanceNow(), clientClearanceIdentity)
	if e != nil || r.Clearance.Reference != "MDCC-actual" || r.Source.Quantity != "1.000001" || r.Version != 1 {
		t.Fatal(e)
	}
	f, e := ClientClearanceClearedFact(&r, s)
	if e != nil || f == nil {
		t.Fatal(e)
	}
	f.Source.Blockers = append(f.Source.Blockers, "Caller")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("mutable fact alias")
	}
	q.Version = 1
	if _, e = ClientClearanceApply(&r, clientClearanceOwner(), clientClearanceCaps(), q, s, clientClearanceNow(), clientClearanceIdentity); e == nil {
		t.Fatal("cleared evidence overwritten")
	}
	next, e := ClientClearanceApply(&r, clientClearanceOwner(), clientClearanceCaps(), ClientClearanceCommand{Version: 1, Action: "revise", Reason: "Correct actual client MDCC"}, s, clientClearanceNow(), clientClearanceIdentity)
	if e != nil || next.Revision != 2 || next.Clearance.Status != "pending" || next.Clearance.Reference != "" || r.Clearance.Reference != "MDCC-actual" {
		t.Fatal("revision reset/retention", e)
	}
	if _, e = ClientClearanceClearedFact(&next, s); e == nil {
		t.Fatal("pending fact cleared")
	}
	changed := s
	changed.QualityVersion++
	if _, e = ClientClearanceClearedFact(&r, changed); e != ClientClearanceStale {
		t.Fatal("old quality approved dispatch", e)
	}
	if _, e = ClientClearanceApply(&r, clientClearanceOwner(), clientClearanceCaps(), ClientClearanceCommand{Version: 1, Action: "revise", Reason: "Review current quality"}, changed, clientClearanceNow(), clientClearanceIdentity); e != nil {
		t.Fatal(e)
	}
	s.Available = false
	if _, e = ClientClearanceClearedFact(&r, s); e == nil {
		t.Fatal("held source cleared")
	}
}
func TestClientClearanceClientClearanceMissingIdentityDateBoundsAndCommands(t *testing.T) {
	for _, mutate := range []func(*ClientClearanceClearanceInput){func(c *ClientClearanceClearanceInput) { c.Reference = " " }, func(c *ClientClearanceClearanceInput) { c.Date = "" }, func(c *ClientClearanceClearanceInput) { c.Date = "2026-02-30" }, func(c *ClientClearanceClearanceInput) { c.Date = "2026-10-09" }, func(c *ClientClearanceClearanceInput) { c.Date = "2026-10-12" }, func(c *ClientClearanceClearanceInput) { c.Authority = strings.Repeat("x", 301) }, func(c *ClientClearanceClearanceInput) { c.Remarks = strings.Repeat("x", 4001) }, func(c *ClientClearanceClearanceInput) { c.Status = "approved" }} {
		c := clientClearanceClear()
		mutate(&c)
		if _, e := ClientClearanceApply(nil, clientClearanceOwner(), clientClearanceCaps(), ClientClearanceCommand{Action: "record", Reason: "Actual MDCC", Clearance: &c}, clientClearanceSource(), clientClearanceNow(), clientClearanceIdentity); e == nil {
			t.Fatal("invalid client evidence passed", c)
		}
	}
	pending := ClientClearanceClearanceInput{Status: "pending"}
	r, e := ClientClearanceApply(nil, clientClearanceOwner(), clientClearanceCaps(), ClientClearanceCommand{Action: "record", Reason: "Prepare", Clearance: &pending}, clientClearanceSource(), clientClearanceNow(), clientClearanceIdentity)
	if e != nil {
		t.Fatal(e)
	}
	if e = ClientClearanceCan(&r, clientClearanceOwner(), clientClearanceCaps(), "revise", clientClearanceSource()); e == nil {
		t.Fatal("unnecessary pending revision")
	}
	c := clientClearanceCaps()
	c.Work = false
	if _, e = ClientClearanceApply(nil, clientClearanceOwner(), c, ClientClearanceCommand{Action: "record", Reason: "x", Clearance: &pending}, clientClearanceSource(), clientClearanceNow(), clientClearanceIdentity); e != ClientClearanceForbidden {
		t.Fatal("unassigned write", e)
	}
	for _, q := range []ClientClearanceCommand{{Action: "record", Reason: "x"}, {Action: "revise", Reason: "x", Clearance: &pending}, {Action: "record", Reason: "", Clearance: &pending}, {Version: -1, Action: "record", Reason: "x", Clearance: &pending}} {
		if _, e := ClientClearanceValidate(q); e == nil {
			t.Fatal("invalid command passed")
		}
	}
}
