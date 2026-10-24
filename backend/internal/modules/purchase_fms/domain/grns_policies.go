package domain

import (
	"math/big"
	"regexp"
	"strings"
)

var gRNsDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$`)

func gRNsQuantity(f, s string, positive bool) (*big.Rat, string, error) {
	s = strings.TrimSpace(s)
	if s == "" && !positive {
		s = "0"
	}
	if !gRNsDecimal.MatchString(s) {
		return nil, s, GRNsInvalid(f, "Use an exact nonnegative quantity with at most six decimal places")
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok || positive && v.Sign() <= 0 {
		return nil, s, GRNsInvalid(f, "Quantity must be positive")
	}
	return v, s, nil
}
func (s GRNsSource) Guard() error {
	if !s.Available || len(s.Blockers) > 0 {
		return GRNsInvalid("source", "Resolve current quality, IRN and project/package blockers first")
	}
	for _, id := range []string{s.ProjectID, s.PackageID, s.ReceiptID, s.DispatchID, s.ClearanceID, s.QualityID, s.ReportID, s.OrderID, s.InspectionID} {
		if e := GRNsID(id); e != nil {
			return e
		}
	}
	if s.ReceiptVersion < 1 || s.ReceiptRevision < 1 || !GRNsDate(s.ArrivalDate) || s.ArrivalDate < s.DispatchDate || s.ReceivingOfficer == "" || s.PackagesReceived < 0 || s.DispatchVersion < 1 || s.DispatchRevision < 1 || !GRNsDate(s.DispatchDate) || s.DispatchDate < s.ClearanceDate || s.LRNumber == "" || s.Transporter == "" || s.ClearanceVersion < 1 || s.ClearanceRevision < 1 || s.Reference == "" || !GRNsDate(s.ClearanceDate) || s.ClearanceDate < s.IssueDate || s.QualityVersion < 1 || s.QualityRevision < 1 || s.ReportVersion < 1 || s.ReportRevision < 1 || s.InspectionVersion < 1 || s.OrderVersion < 2 || !GRNsDate(s.IssueDate) || s.IRNumber == "" || s.IRNNumber == "" || s.Number == "" || s.SupplierName == "" || s.Description == "" || s.UOM == "" {
		return GRNsInvalid("source", "Use the actual current physical site receipt and its clearance evidence")
	}
	_, _, e := gRNsQuantity("source", s.Quantity, true)
	return e
}
func GRNsAccessGuard(a Principal, c GRNsAccess, write bool) error {
	if !a.Active || a.ID == "" {
		return GRNsUnauthorized
	}
	if !c.Read || write && (!c.Write || !c.Work) {
		return GRNsForbidden
	}
	return nil
}
func GRNsValidate(q GRNsCommand) (GRNsCommand, error) {
	if q.Version < 0 {
		return q, GRNsInvalid("version", "Use the current nonnegative grn version")
	}
	var e error
	q.Reason, e = GRNsText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	switch q.Action {
	case "record":
		if q.GRN == nil {
			return q, GRNsInvalid("grn", "Supply the grn details")
		}
	case "revise":
		if q.GRN != nil {
			return q, GRNsInvalid("grn", "Revision carries only its version and reason")
		}
	default:
		return q, GRNsInvalid("action", "Choose record or revise")
	}
	return q, nil
}
func gRNsGrn(q GRNsGRNInput, s GRNsSource, today string) (GRNsGRN, error) {
	out := GRNsGRN{GRNsGRNInput: q, UOM: s.UOM}
	if out.Status != "pending" && out.Status != "issued" {
		return out, GRNsInvalid("status", "Choose pending or issued")
	}
	for _, v := range []struct {
		f        string
		p        *string
		n        int
		required bool
	}{
		{"grn_number", &out.Number, 200, out.Status == "issued"}, {"store_keeper_name", &out.StoreKeeper, 300, false},
		{"remarks", &out.Remarks, 4000, false}, {"warranty_ref", &out.WarrantyReference, 200, false}, {"file_name", &out.DocumentReference, 500, false},
	} {
		var e error
		*v.p, e = GRNsText(v.f, *v.p, v.n, v.required)
		if e != nil {
			return out, e
		}
	}
	if out.Date != "" && (!GRNsDate(out.Date) || out.Date < s.ArrivalDate || out.Date > today) || out.Status == "issued" && out.Date == "" {
		return out, GRNsInvalid("grn_date", "Use the actual GRN date on or after site arrival and not in the future")
	}
	if out.WarrantyValidUntil != "" && !GRNsDate(out.WarrantyValidUntil) {
		return out, GRNsInvalid("warranty_valid_until", "Use a real warranty validity date")
	}
	received, normalized, e := gRNsQuantity("received_qty", out.Received, out.Status == "issued")
	if e != nil {
		return out, e
	}
	out.Received = normalized
	accepted, normalized, e := gRNsQuantity("accepted_qty", out.Accepted, false)
	if e != nil {
		return out, e
	}
	out.Accepted = normalized
	rejected, normalized, e := gRNsQuantity("rejected_qty", out.Rejected, false)
	if e != nil {
		return out, e
	}
	out.Rejected = normalized
	if out.Status == "issued" {
		allowance, _, e := gRNsQuantity("source", s.Quantity, true)
		if e != nil {
			return out, e
		}
		if received.Cmp(allowance) > 0 {
			return out, GRNsInvalid("received_qty", "Received quantity cannot exceed the current cleared allowance")
		}
		if new(big.Rat).Add(accepted, rejected).Cmp(received) > 0 {
			return out, GRNsInvalid("accepted_qty", "Accepted and rejected quantities together cannot exceed received quantity")
		}
	}
	return out, nil
}
func GRNsCan(r *GRNsRecord, a Principal, c GRNsAccess, action string, s GRNsSource) error {
	if e := GRNsAccessGuard(a, c, true); e != nil {
		return e
	}
	if e := s.Guard(); e != nil {
		return e
	}
	if action != "record" && action != "revise" {
		return GRNsInvalid("action", "Choose record or revise")
	}
	if r == nil {
		if action != "record" {
			return GRNsInvalid("grn", "Record the grn before starting a revision")
		}
		return nil
	}
	if !r.Active || r.ProjectID != s.ProjectID || r.PackageID != s.PackageID {
		return GRNsMissing
	}
	if action == "record" && (r.Source.GRNsSourceFacts != s.GRNsSourceFacts || r.GRN.Status == "issued") {
		return GRNsStale
	}
	if action == "revise" && r.GRN.Status != "issued" && r.Source.GRNsSourceFacts == s.GRNsSourceFacts {
		return GRNsInvalid("revision", "Edit the current pending GRN instead")
	}
	return nil
}
func GRNsApply(base *GRNsRecord, a Principal, c GRNsAccess, q GRNsCommand, s GRNsSource, now GRNsTimestamp, id string) (GRNsRecord, error) {
	empty := GRNsRecord{}
	q, e := GRNsValidate(q)
	if e != nil {
		return empty, e
	}
	if e = GRNsCan(base, a, c, q.Action, s); e != nil {
		return empty, e
	}
	out := GRNsRecord{ID: id, ProjectID: s.ProjectID, PackageID: s.PackageID, Revision: 1, Source: s, Active: true, CreatedAt: now, CreatedBy: a.ID}
	if base != nil {
		if q.Version != base.Version {
			return empty, GRNsStale
		}
		out = GRNsCopy(*base)
	} else if q.Version != 0 {
		return empty, GRNsStale
	}
	for _, id := range []string{out.ID, a.ID} {
		if e = GRNsID(id); e != nil {
			return empty, e
		}
	}
	if q.Action == "revise" {
		out.Revision++
		out.Source = s
		out.GRN = GRNsGRN{GRNsGRNInput: GRNsGRNInput{Status: "pending", Received: "0", Accepted: "0", Rejected: "0"}, UOM: s.UOM}
	} else {
		out.GRN, e = gRNsGrn(*q.GRN, s, GRNsToday(now))
		if e != nil {
			return empty, e
		}
	}
	out.Version = q.Version + 1
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}
func GRNsIssuedFact(r *GRNsRecord, s GRNsSource) (*GRNsRecord, error) {
	if e := s.Guard(); e != nil {
		return nil, e
	}
	if r == nil || !r.Active {
		return nil, GRNsMissing
	}
	if r.ProjectID != s.ProjectID || r.PackageID != s.PackageID || r.Source.GRNsSourceFacts != s.GRNsSourceFacts {
		return nil, GRNsStale
	}
	if r.Version < 1 || r.Revision < 1 || r.GRN.UOM != s.UOM || r.GRN.Status != "issued" {
		return nil, GRNsInvalid("grn", "Current issued GRN is required")
	}
	if _, e := gRNsGrn(r.GRN.GRNsGRNInput, s, GRNsToday(r.UpdatedAt)); e != nil {
		return nil, e
	}
	out := GRNsCopy(*r)
	return &out, nil
}
