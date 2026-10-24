package domain

import (
	"strings"
	"testing"
	"time"
)

const siteReceiptsProject = "11111111-1111-4111-8111-111111111111"
const siteReceiptsPkg = "22222222-2222-4222-8222-222222222222"
const siteReceiptsActorID = "33333333-3333-4333-8333-333333333333"
const siteReceiptsIdentity = "44444444-4444-4444-8444-444444444444"

func siteReceiptsSource() SiteReceiptsSource {
	return SiteReceiptsSource{Available: true, Blockers: []string{}, SiteReceiptsSourceFacts: SiteReceiptsSourceFacts{DispatchID: siteReceiptsIdentity, DispatchVersion: 6, DispatchRevision: 3, DispatchDate: "2026-10-10", LRNumber: "LR-1", Transporter: "Carrier", ClearanceID: siteReceiptsIdentity, ClearanceVersion: 6, ClearanceRevision: 3, Reference: "MDCC-1", ClearanceDate: "2026-10-10", ProjectID: siteReceiptsProject, PackageID: siteReceiptsPkg, QualityID: siteReceiptsIdentity, QualityVersion: 23, QualityRevision: 2, ReportID: siteReceiptsIdentity, ReportVersion: 8, ReportRevision: 4, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: siteReceiptsIdentity, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: siteReceiptsIdentity, InspectionVersion: 8}}
}
func siteReceiptsOwner() Principal { return Principal{ID: siteReceiptsActorID, Active: true} }
func siteReceiptsCaps() SiteReceiptsAccess {
	return SiteReceiptsAccess{Read: true, Write: true, Scoped: true, Work: true}
}
func siteReceiptsNow() SiteReceiptsTimestamp { return time.Date(2026, 10, 11, 5, 0, 0, 0, time.UTC) }
func siteReceiptsDelivered() SiteReceiptsReceiptInput {
	return SiteReceiptsReceiptInput{Status: "delivered_at_site", ArrivalDate: "2026-10-11", ReceivingOfficer: "Actual officer", PackagesCondition: "quarantine", TransitDamageNotes: "Actual damage"}
}
func TestSiteReceiptsPhysicalReceiptAndCurrentDispatchRevision(t *testing.T) {
	s := siteReceiptsSource()
	in := siteReceiptsDelivered()
	q := SiteReceiptsCommand{Action: "record", Reason: "Actual receipt", Receipt: &in}
	r, e := SiteReceiptsApply(nil, siteReceiptsOwner(), siteReceiptsCaps(), q, s, siteReceiptsNow(), siteReceiptsIdentity)
	if e != nil || r.Receipt.PackagesReceived != 0 || r.Receipt.PackagesCondition != "quarantine" || r.Version != 1 {
		t.Fatal(e, r)
	}
	f, e := SiteReceiptsDeliveredFact(&r, s)
	if e != nil || f == nil || f.Receipt.PackagesCondition != "quarantine" {
		t.Fatal("physical receipt must retain damage, without stock acceptance", e)
	}
	f.Source.Blockers = append(f.Source.Blockers, "Caller")
	if len(r.Source.Blockers) != 0 {
		t.Fatal("mutable fact alias")
	}
	q.Version = 1
	if _, e = SiteReceiptsApply(&r, siteReceiptsOwner(), siteReceiptsCaps(), q, s, siteReceiptsNow(), siteReceiptsIdentity); e != SiteReceiptsStale {
		t.Fatal("final overwritten", e)
	}
	changed := s
	changed.DispatchVersion++
	if _, e = SiteReceiptsDeliveredFact(&r, changed); e != SiteReceiptsStale {
		t.Fatal("obsolete dispatch still current", e)
	}
	next, e := SiteReceiptsApply(&r, siteReceiptsOwner(), siteReceiptsCaps(), SiteReceiptsCommand{Version: 1, Action: "revise", Reason: "Review actual corrected dispatch"}, changed, siteReceiptsNow(), siteReceiptsIdentity)
	if e != nil || next.Revision != 2 || next.Receipt.Status != "pending" || next.Receipt.PackagesCondition != "not_checked" || next.Receipt.ReceivingOfficer != "" || r.Receipt.ReceivingOfficer == "" {
		t.Fatal("revision reset/retention", e)
	}
	if _, e = SiteReceiptsDeliveredFact(&next, changed); e == nil {
		t.Fatal("pending treated delivered")
	}
	s.Available = false
	if _, e = SiteReceiptsDeliveredFact(&r, s); e == nil {
		t.Fatal("held receipt still current")
	}
}
func TestSiteReceiptsReceiptRequiredFieldsCountsConditionsAndDates(t *testing.T) {
	for _, mutate := range []func(*SiteReceiptsReceiptInput){
		func(r *SiteReceiptsReceiptInput) { r.ArrivalDate = "" }, func(r *SiteReceiptsReceiptInput) { r.ReceivingOfficer = " " },
		func(r *SiteReceiptsReceiptInput) { r.ArrivalDate = "2026-02-30" }, func(r *SiteReceiptsReceiptInput) { r.ArrivalDate = "2026-10-09" }, func(r *SiteReceiptsReceiptInput) { r.ArrivalDate = "2026-10-12" },
		func(r *SiteReceiptsReceiptInput) { r.PackagesReceived = -1 }, func(r *SiteReceiptsReceiptInput) { r.PackagesCondition = "accepted_stock" }, func(r *SiteReceiptsReceiptInput) { r.Status = "accepted" },
		func(r *SiteReceiptsReceiptInput) { r.ReceivingOfficer = strings.Repeat("x", 301) }, func(r *SiteReceiptsReceiptInput) { r.TransitDamageNotes = strings.Repeat("x", 4001) }, func(r *SiteReceiptsReceiptInput) { r.GateEntryReference = strings.Repeat("x", 201) },
	} {
		r := siteReceiptsDelivered()
		mutate(&r)
		if _, e := SiteReceiptsApply(nil, siteReceiptsOwner(), siteReceiptsCaps(), SiteReceiptsCommand{Action: "record", Reason: "Actual", Receipt: &r}, siteReceiptsSource(), siteReceiptsNow(), siteReceiptsIdentity); e == nil {
			t.Fatal("invalid receipt passed", r)
		}
	}
	for _, status := range []string{"pending", "in_transit"} {
		r := SiteReceiptsReceiptInput{Status: status}
		out, e := SiteReceiptsApply(nil, siteReceiptsOwner(), siteReceiptsCaps(), SiteReceiptsCommand{Action: "record", Reason: "Prepare", Receipt: &r}, siteReceiptsSource(), siteReceiptsNow(), siteReceiptsIdentity)
		if e != nil || out.Receipt.PackagesCondition != "not_checked" {
			t.Fatal("invented draft evidence", e)
		}
		if SiteReceiptsCan(&out, siteReceiptsOwner(), siteReceiptsCaps(), "revise", siteReceiptsSource()) == nil {
			t.Fatal("unnecessary draft revision")
		}
	}
	c := siteReceiptsCaps()
	c.Work = false
	r := siteReceiptsDelivered()
	if _, e := SiteReceiptsApply(nil, siteReceiptsOwner(), c, SiteReceiptsCommand{Action: "record", Reason: "Actual", Receipt: &r}, siteReceiptsSource(), siteReceiptsNow(), siteReceiptsIdentity); e != SiteReceiptsForbidden {
		t.Fatal("unassigned write", e)
	}
}
