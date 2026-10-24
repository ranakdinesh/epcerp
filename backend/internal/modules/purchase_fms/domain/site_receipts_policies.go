package domain

import (
	"math/big"
	"regexp"
	"strings"
)

var siteReceiptsDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$`)

func siteReceiptsQuantity(f, s string, positive bool) (*big.Rat, string, error) {
	s = strings.TrimSpace(s)
	if s == "" && !positive {
		s = "0"
	}
	if !siteReceiptsDecimal.MatchString(s) {
		return nil, s, SiteReceiptsInvalid(f, "Use an exact nonnegative quantity with at most six decimal places")
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok || positive && v.Sign() <= 0 {
		return nil, s, SiteReceiptsInvalid(f, "Cleared quantity must be positive")
	}
	return v, s, nil
}
func (s SiteReceiptsSource) Guard() error {
	if !s.Available || len(s.Blockers) > 0 {
		return SiteReceiptsInvalid("source", "Resolve current quality, IRN and project/package blockers first")
	}
	for _, id := range []string{s.ProjectID, s.PackageID, s.DispatchID, s.ClearanceID, s.QualityID, s.ReportID, s.OrderID, s.InspectionID} {
		if e := SiteReceiptsID(id); e != nil {
			return e
		}
	}
	if s.DispatchVersion < 1 || s.DispatchRevision < 1 || !SiteReceiptsDate(s.DispatchDate) || s.DispatchDate < s.ClearanceDate || s.LRNumber == "" || s.Transporter == "" || s.ClearanceVersion < 1 || s.ClearanceRevision < 1 || s.Reference == "" || !SiteReceiptsDate(s.ClearanceDate) || s.ClearanceDate < s.IssueDate || s.QualityVersion < 1 || s.QualityRevision < 1 || s.ReportVersion < 1 || s.ReportRevision < 1 || s.InspectionVersion < 1 || s.OrderVersion < 2 || !SiteReceiptsDate(s.IssueDate) || s.IRNumber == "" || s.IRNNumber == "" || s.Number == "" || s.SupplierName == "" || s.Description == "" || s.UOM == "" {
		return SiteReceiptsInvalid("source", "Use the actual current material dispatch and its clearance evidence")
	}
	_, _, e := siteReceiptsQuantity("source", s.Quantity, true)
	return e
}
func SiteReceiptsAccessGuard(a Principal, c SiteReceiptsAccess, write bool) error {
	if !a.Active || a.ID == "" {
		return SiteReceiptsUnauthorized
	}
	if !c.Read || write && (!c.Write || !c.Work) {
		return SiteReceiptsForbidden
	}
	return nil
}
func SiteReceiptsValidate(q SiteReceiptsCommand) (SiteReceiptsCommand, error) {
	if q.Version < 0 {
		return q, SiteReceiptsInvalid("version", "Use the current nonnegative receipt version")
	}
	var e error
	q.Reason, e = SiteReceiptsText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	switch q.Action {
	case "record":
		if q.Receipt == nil {
			return q, SiteReceiptsInvalid("receipt", "Supply the receipt details")
		}
	case "revise":
		if q.Receipt != nil {
			return q, SiteReceiptsInvalid("receipt", "Revision carries only its version and reason")
		}
	default:
		return q, SiteReceiptsInvalid("action", "Choose record or revise")
	}
	return q, nil
}
func siteReceiptsReceipt(q SiteReceiptsReceiptInput, s SiteReceiptsSource, today string) (SiteReceiptsReceipt, error) {
	out := q
	if out.Status != "pending" && out.Status != "in_transit" && out.Status != "delivered_at_site" {
		return out, SiteReceiptsInvalid("status", "Choose pending, in transit or delivered at site")
	}
	if out.PackagesCondition == "" {
		out.PackagesCondition = "not_checked"
	}
	switch out.PackagesCondition {
	case "not_checked", "intact", "minor_damage", "quarantine":
	default:
		return out, SiteReceiptsInvalid("packages_condition", "Choose the actual package condition")
	}
	if out.PackagesReceived < 0 {
		return out, SiteReceiptsInvalid("packages_received", "Package count cannot be negative")
	}
	for _, v := range []struct {
		f        string
		p        *string
		n        int
		required bool
	}{
		{"receiving_officer", &out.ReceivingOfficer, 300, out.Status == "delivered_at_site"},
		{"transit_damage_notes", &out.TransitDamageNotes, 4000, false},
		{"gate_entry_ref", &out.GateEntryReference, 200, false},
	} {
		var e error
		*v.p, e = SiteReceiptsText(v.f, *v.p, v.n, v.required)
		if e != nil {
			return out, e
		}
	}
	if out.ArrivalDate != "" && (!SiteReceiptsDate(out.ArrivalDate) || out.ArrivalDate < s.DispatchDate || out.ArrivalDate > today) || out.Status == "delivered_at_site" && out.ArrivalDate == "" {
		return out, SiteReceiptsInvalid("arrival_date", "Use the actual site arrival date on or after dispatch and not in the future")
	}
	return out, nil
}
func SiteReceiptsCan(r *SiteReceiptsRecord, a Principal, c SiteReceiptsAccess, action string, s SiteReceiptsSource) error {
	if e := SiteReceiptsAccessGuard(a, c, true); e != nil {
		return e
	}
	if e := s.Guard(); e != nil {
		return e
	}
	if action != "record" && action != "revise" {
		return SiteReceiptsInvalid("action", "Choose record or revise")
	}
	if r == nil {
		if action != "record" {
			return SiteReceiptsInvalid("receipt", "Record the receipt before starting a revision")
		}
		return nil
	}
	if !r.Active || r.ProjectID != s.ProjectID || r.PackageID != s.PackageID {
		return SiteReceiptsMissing
	}
	if action == "record" && (r.Source.SiteReceiptsSourceFacts != s.SiteReceiptsSourceFacts || r.Receipt.Status == "delivered_at_site") {
		return SiteReceiptsStale
	}
	if action == "revise" && r.Receipt.Status != "delivered_at_site" && r.Source.SiteReceiptsSourceFacts == s.SiteReceiptsSourceFacts {
		return SiteReceiptsInvalid("revision", "Edit the current pending or in-transit receipt instead")
	}
	return nil
}
func SiteReceiptsApply(base *SiteReceiptsRecord, a Principal, c SiteReceiptsAccess, q SiteReceiptsCommand, s SiteReceiptsSource, now SiteReceiptsTimestamp, id string) (SiteReceiptsRecord, error) {
	empty := SiteReceiptsRecord{}
	q, e := SiteReceiptsValidate(q)
	if e != nil {
		return empty, e
	}
	if e = SiteReceiptsCan(base, a, c, q.Action, s); e != nil {
		return empty, e
	}
	out := SiteReceiptsRecord{ID: id, ProjectID: s.ProjectID, PackageID: s.PackageID, Revision: 1, Source: s, Active: true, CreatedAt: now, CreatedBy: a.ID}
	if base != nil {
		if q.Version != base.Version {
			return empty, SiteReceiptsStale
		}
		out = SiteReceiptsCopy(*base)
	} else if q.Version != 0 {
		return empty, SiteReceiptsStale
	}
	for _, id := range []string{out.ID, a.ID} {
		if e = SiteReceiptsID(id); e != nil {
			return empty, e
		}
	}
	if q.Action == "revise" {
		out.Revision++
		out.Source = s
		out.Receipt = SiteReceiptsReceipt{Status: "pending", PackagesCondition: "not_checked"}
	} else {
		out.Receipt, e = siteReceiptsReceipt(*q.Receipt, s, SiteReceiptsToday(now))
		if e != nil {
			return empty, e
		}
	}
	out.Version = q.Version + 1
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}
func SiteReceiptsDeliveredFact(r *SiteReceiptsRecord, s SiteReceiptsSource) (*SiteReceiptsRecord, error) {
	if e := s.Guard(); e != nil {
		return nil, e
	}
	if r == nil || !r.Active {
		return nil, SiteReceiptsMissing
	}
	if r.ProjectID != s.ProjectID || r.PackageID != s.PackageID || r.Source.SiteReceiptsSourceFacts != s.SiteReceiptsSourceFacts {
		return nil, SiteReceiptsStale
	}
	if r.Version < 1 || r.Revision < 1 || r.Receipt.Status != "delivered_at_site" {
		return nil, SiteReceiptsInvalid("receipt", "Current physical site receipt is required")
	}
	if _, e := siteReceiptsReceipt(r.Receipt, s, SiteReceiptsToday(r.UpdatedAt)); e != nil {
		return nil, e
	}
	out := SiteReceiptsCopy(*r)
	return &out, nil
}
