package domain

import (
	"strings"
	"testing"
	"time"
)

const engineerConfirmationsProject = "11111111-1111-4111-8111-111111111111"
const engineerConfirmationsPkg = "22222222-2222-4222-8222-222222222222"
const engineerConfirmationsActorID = "33333333-3333-4333-8333-333333333333"
const engineerConfirmationsIdentity = "44444444-4444-4444-8444-444444444444"

func engineerConfirmationsSource() EngineerConfirmationsSource {
	return EngineerConfirmationsSource{Available: true, Blockers: []string{}, EngineerConfirmationsSourceFacts: EngineerConfirmationsSourceFacts{GRNID: engineerConfirmationsIdentity, GRNVersion: 6, GRNRevision: 3, GRNNumber: "GRN-1", GRNDate: "2026-10-10", Received: "1.000001", Accepted: "1", Rejected: "0.000001", ReceiptID: engineerConfirmationsIdentity, ReceiptVersion: 6, ReceiptRevision: 3, ArrivalDate: "2026-10-10", ReceivingOfficer: "Actual officer", PackagesCondition: "quarantine", DispatchID: engineerConfirmationsIdentity, DispatchVersion: 6, DispatchRevision: 3, DispatchDate: "2026-10-10", LRNumber: "LR-1", Transporter: "Carrier", ClearanceID: engineerConfirmationsIdentity, ClearanceVersion: 6, ClearanceRevision: 3, Reference: "MDCC-1", ClearanceDate: "2026-10-10", ProjectID: engineerConfirmationsProject, PackageID: engineerConfirmationsPkg, QualityID: engineerConfirmationsIdentity, QualityVersion: 23, QualityRevision: 2, ReportID: engineerConfirmationsIdentity, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: engineerConfirmationsIdentity, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: engineerConfirmationsIdentity, InspectionVersion: 8}}
}
func engineerConfirmationsOwner() Principal {
	return Principal{ID: engineerConfirmationsActorID, Active: true}
}
func engineerConfirmationsCaps() EngineerConfirmationsAccess {
	return EngineerConfirmationsAccess{Read: true, Write: true, Scoped: true, Work: true}
}
func engineerConfirmationsNow() EngineerConfirmationsTimestamp {
	return time.Date(2026, 10, 11, 5, 0, 0, 0, time.UTC)
}
func engineerConfirmationsConfirmed() EngineerConfirmationsConfirmationInput {
	return EngineerConfirmationsConfirmationInput{Status: "confirmed", EngineerName: "Actual engineer", Date: "2026-10-11", Designation: "Project Engineer", MBookReference: "MB-actual", WorkPackageReference: "Actual work package", Remarks: "Actual inspection"}
}
func TestEngineerConfirmationsEngineerConfirmationCurrentGRNRevision(t *testing.T) {
	s := engineerConfirmationsSource()
	in := engineerConfirmationsConfirmed()
	q := EngineerConfirmationsCommand{Action: "record", Reason: "Actual confirmation", Confirmation: &in}
	r, e := EngineerConfirmationsApply(nil, engineerConfirmationsOwner(), engineerConfirmationsCaps(), q, s, engineerConfirmationsNow(), engineerConfirmationsIdentity)
	if e != nil || r.Confirmation.EngineerName != "Actual engineer" || r.Confirmation.MBookReference != "MB-actual" {
		t.Fatal(e, r)
	}
	f, e := EngineerConfirmationsConfirmedFact(&r, s)
	if e != nil || f == nil {
		t.Fatal(e)
	}
	f.Source.Blockers = append(f.Source.Blockers, "Caller")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("mutable fact alias")
	}
	q.Version = 1
	if _, e = EngineerConfirmationsApply(&r, engineerConfirmationsOwner(), engineerConfirmationsCaps(), q, s, engineerConfirmationsNow(), engineerConfirmationsIdentity); e != EngineerConfirmationsStale {
		t.Fatal("confirmed overwritten", e)
	}
	changed := s
	changed.GRNVersion++
	if _, e = EngineerConfirmationsConfirmedFact(&r, changed); e != EngineerConfirmationsStale {
		t.Fatal("obsolete GRN", e)
	}
	next, e := EngineerConfirmationsApply(&r, engineerConfirmationsOwner(), engineerConfirmationsCaps(), EngineerConfirmationsCommand{Version: 1, Action: "revise", Reason: "Review corrected GRN"}, changed, engineerConfirmationsNow(), engineerConfirmationsIdentity)
	if e != nil || next.Revision != 2 || next.Confirmation.Status != "pending" || next.Confirmation.EngineerName != "" || next.Confirmation.MBookReference != "" || r.Confirmation.MBookReference == "" {
		t.Fatal("revision reset/retention", e)
	}
	if _, e = EngineerConfirmationsConfirmedFact(&next, changed); e == nil {
		t.Fatal("pending confirmed")
	}
	s.Available = false
	if _, e = EngineerConfirmationsConfirmedFact(&r, s); e == nil {
		t.Fatal("held confirmed")
	}
}
func TestEngineerConfirmationsEngineerRequiredActualDateOptionalReferencesAndScope(t *testing.T) {
	for _, mutate := range []func(*EngineerConfirmationsConfirmationInput){func(r *EngineerConfirmationsConfirmationInput) { r.EngineerName = " " }, func(r *EngineerConfirmationsConfirmationInput) { r.Date = "" }, func(r *EngineerConfirmationsConfirmationInput) { r.Date = "2026-02-30" }, func(r *EngineerConfirmationsConfirmationInput) { r.Date = "2026-10-09" }, func(r *EngineerConfirmationsConfirmationInput) { r.Date = "2026-10-12" }, func(r *EngineerConfirmationsConfirmationInput) { r.Status = "approved" }, func(r *EngineerConfirmationsConfirmationInput) { r.EngineerName = strings.Repeat("x", 301) }, func(r *EngineerConfirmationsConfirmationInput) { r.MBookReference = strings.Repeat("x", 201) }, func(r *EngineerConfirmationsConfirmationInput) { r.Remarks = strings.Repeat("x", 4001) }} {
		r := engineerConfirmationsConfirmed()
		mutate(&r)
		if _, e := EngineerConfirmationsApply(nil, engineerConfirmationsOwner(), engineerConfirmationsCaps(), EngineerConfirmationsCommand{Action: "record", Reason: "Actual", Confirmation: &r}, engineerConfirmationsSource(), engineerConfirmationsNow(), engineerConfirmationsIdentity); e == nil {
			t.Fatal("invalid confirmation passed", r)
		}
	}
	r := engineerConfirmationsConfirmed()
	r.Designation = ""
	r.MBookReference = ""
	r.WorkPackageReference = ""
	r.Remarks = ""
	s := engineerConfirmationsSource()
	s.Accepted = "0"
	s.Rejected = "0"
	if _, e := EngineerConfirmationsApply(nil, engineerConfirmationsOwner(), engineerConfirmationsCaps(), EngineerConfirmationsCommand{Action: "record", Reason: "Actual review", Confirmation: &r}, s, engineerConfirmationsNow(), engineerConfirmationsIdentity); e != nil {
		t.Fatal("invented optional references or accepted-stock gate", e)
	}
	pending := EngineerConfirmationsConfirmationInput{Status: "pending"}
	out, e := EngineerConfirmationsApply(nil, engineerConfirmationsOwner(), engineerConfirmationsCaps(), EngineerConfirmationsCommand{Action: "record", Reason: "Prepare", Confirmation: &pending}, engineerConfirmationsSource(), engineerConfirmationsNow(), engineerConfirmationsIdentity)
	if e != nil || out.Confirmation.EngineerName != "" {
		t.Fatal("invented draft evidence", e)
	}
	if EngineerConfirmationsCan(&out, engineerConfirmationsOwner(), engineerConfirmationsCaps(), "revise", engineerConfirmationsSource()) == nil {
		t.Fatal("unnecessary pending revision")
	}
	c := engineerConfirmationsCaps()
	c.Work = false
	if _, e = EngineerConfirmationsApply(nil, engineerConfirmationsOwner(), c, EngineerConfirmationsCommand{Action: "record", Reason: "Actual", Confirmation: &r}, engineerConfirmationsSource(), engineerConfirmationsNow(), engineerConfirmationsIdentity); e != EngineerConfirmationsForbidden {
		t.Fatal("unassigned write", e)
	}
}
