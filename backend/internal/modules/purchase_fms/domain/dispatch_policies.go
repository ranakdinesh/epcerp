package domain

import (
	"math/big"
	"regexp"
	"strings"
)

var dispatchDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$`)

func dispatchQuantity(f, s string, positive bool) (*big.Rat, string, error) {
	s = strings.TrimSpace(s)
	if s == "" && !positive {
		s = "0"
	}
	if !dispatchDecimal.MatchString(s) {
		return nil, s, DispatchInvalid(f, "Use an exact nonnegative quantity with at most six decimal places")
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok || positive && v.Sign() <= 0 {
		return nil, s, DispatchInvalid(f, "Cleared quantity must be positive")
	}
	return v, s, nil
}
func (s DispatchSource) Guard() error {
	if !s.Available || len(s.Blockers) > 0 {
		return DispatchInvalid("source", "Resolve current quality, IRN and project/package blockers first")
	}
	for _, id := range []string{s.ProjectID, s.PackageID, s.ClearanceID, s.QualityID, s.ReportID, s.OrderID, s.InspectionID} {
		if e := DispatchID(id); e != nil {
			return e
		}
	}
	if s.ClearanceVersion < 1 || s.ClearanceRevision < 1 || s.Reference == "" || !DispatchDate(s.ClearanceDate) || s.ClearanceDate < s.IssueDate || s.QualityVersion < 1 || s.QualityRevision < 1 || s.ReportVersion < 1 || s.ReportRevision < 1 || s.InspectionVersion < 1 || s.OrderVersion < 2 || !DispatchDate(s.IssueDate) || s.IRNumber == "" || s.IRNNumber == "" || s.Number == "" || s.SupplierName == "" || s.Description == "" || s.UOM == "" {
		return DispatchInvalid("source", "Use the actual current verified quality and issued IRN")
	}
	_, _, e := dispatchQuantity("source", s.Quantity, true)
	return e
}
func DispatchAccessGuard(a Principal, c DispatchAccess, write bool) error {
	if !a.Active || a.ID == "" {
		return DispatchUnauthorized
	}
	if !c.Read || write && (!c.Write || !c.Work) {
		return DispatchForbidden
	}
	return nil
}
func DispatchValidate(q DispatchCommand) (DispatchCommand, error) {
	if q.Version < 0 {
		return q, DispatchInvalid("version", "Use the current nonnegative dispatch version")
	}
	var e error
	q.Reason, e = DispatchText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	switch q.Action {
	case "record":
		if q.Dispatch == nil {
			return q, DispatchInvalid("dispatch", "Supply the dispatch details")
		}
	case "revise":
		if q.Dispatch != nil {
			return q, DispatchInvalid("dispatch", "Revision carries only its version and reason")
		}
	default:
		return q, DispatchInvalid("action", "Choose record or revise")
	}
	return q, nil
}
func dispatchDispatch(q DispatchInput, s DispatchSource, today string) (Dispatch, error) {
	out := q
	if q.Status != "pending" && q.Status != "dispatched" {
		return out, DispatchInvalid("status", "Choose pending or dispatched")
	}
	for _, v := range []struct {
		f        string
		p        *string
		n        int
		required bool
	}{{"transporter_name", &out.Transporter, 300, q.Status == "dispatched"}, {"lr_number", &out.LRNumber, 200, q.Status == "dispatched"}, {"vehicle_number", &out.Vehicle, 100, false}, {"eway_bill_number", &out.EwayNumber, 200, false}, {"invoice_number", &out.InvoiceNumber, 200, false}, {"driver_contact", &out.DriverContact, 100, false}, {"file_name", &out.DocumentReference, 500, false}} {
		var e error
		*v.p, e = DispatchText(v.f, *v.p, v.n, v.required)
		if e != nil {
			return out, e
		}
	}
	if out.Date != "" && (!DispatchDate(out.Date) || out.Date < s.ClearanceDate || out.Date > today) || q.Status == "dispatched" && out.Date == "" {
		return out, DispatchInvalid("dispatch_date", "Use the actual dispatch date on or after client clearance and not in the future")
	}
	for _, v := range []struct{ f, s string }{{"lr_date", out.LRDate}, {"invoice_date", out.InvoiceDate}} {
		if v.s != "" && (!DispatchDate(v.s) || v.s > today) {
			return out, DispatchInvalid(v.f, "Use a real nonfuture document date")
		}
	}
	if out.EwayValidUntil != "" && !DispatchDate(out.EwayValidUntil) {
		return out, DispatchInvalid("eway_bill_valid_until", "Use a real e-way bill validity date")
	}
	return out, nil
}
func DispatchCan(r *DispatchRecord, a Principal, c DispatchAccess, action string, s DispatchSource) error {
	if e := DispatchAccessGuard(a, c, true); e != nil {
		return e
	}
	if e := s.Guard(); e != nil {
		return e
	}
	if action != "record" && action != "revise" {
		return DispatchInvalid("action", "Choose record or revise")
	}
	if r == nil {
		if action != "record" {
			return DispatchInvalid("dispatch", "Record the dispatch before starting a revision")
		}
		return nil
	}
	if !r.Active || r.ProjectID != s.ProjectID || r.PackageID != s.PackageID {
		return DispatchMissing
	}
	if action == "record" && (r.Source.DispatchSourceFacts != s.DispatchSourceFacts || r.Dispatch.Status == "dispatched") {
		return DispatchStale
	}
	if action == "revise" && r.Dispatch.Status != "dispatched" && r.Source.DispatchSourceFacts == s.DispatchSourceFacts {
		return DispatchInvalid("revision", "Edit the current pending dispatch instead")
	}
	return nil
}
func DispatchApply(base *DispatchRecord, a Principal, c DispatchAccess, q DispatchCommand, s DispatchSource, now DispatchTimestamp, id string) (DispatchRecord, error) {
	empty := DispatchRecord{}
	q, e := DispatchValidate(q)
	if e != nil {
		return empty, e
	}
	if e = DispatchCan(base, a, c, q.Action, s); e != nil {
		return empty, e
	}
	out := DispatchRecord{ID: id, ProjectID: s.ProjectID, PackageID: s.PackageID, Revision: 1, Source: s, Active: true, CreatedAt: now, CreatedBy: a.ID}
	if base != nil {
		if q.Version != base.Version {
			return empty, DispatchStale
		}
		out = DispatchCopy(*base)
	} else if q.Version != 0 {
		return empty, DispatchStale
	}
	for _, id := range []string{out.ID, a.ID} {
		if e = DispatchID(id); e != nil {
			return empty, e
		}
	}
	if q.Action == "revise" {
		out.Revision++
		out.Source = s
		out.Dispatch = Dispatch{Status: "pending"}
	} else {
		out.Dispatch, e = dispatchDispatch(*q.Dispatch, s, DispatchToday(now))
		if e != nil {
			return empty, e
		}
	}
	out.Version = q.Version + 1
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}
func DispatchedFact(r *DispatchRecord, s DispatchSource) (*DispatchRecord, error) {
	if e := s.Guard(); e != nil {
		return nil, e
	}
	if r == nil || !r.Active {
		return nil, DispatchMissing
	}
	if r.ProjectID != s.ProjectID || r.PackageID != s.PackageID || r.Source.DispatchSourceFacts != s.DispatchSourceFacts {
		return nil, DispatchStale
	}
	if r.Version < 1 || r.Revision < 1 || r.Dispatch.Status != "dispatched" {
		return nil, DispatchInvalid("dispatch", "Current issued Material dispatch is required")
	}
	if _, e := dispatchDispatch(r.Dispatch, s, DispatchToday(r.UpdatedAt)); e != nil {
		return nil, e
	}
	out := DispatchCopy(*r)
	return &out, nil
}
