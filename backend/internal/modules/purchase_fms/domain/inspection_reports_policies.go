package domain

import (
	"math/big"
	"regexp"
	"strings"
)

var inspectionReportsDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$`)

func inspectionReportsQuantity(f, s string, positive bool) (*big.Rat, string, error) {
	s = strings.TrimSpace(s)
	if s == "" && !positive {
		s = "0"
	}
	if !inspectionReportsDecimal.MatchString(s) {
		return nil, s, InspectionReportsInvalid(f, "Use an exact nonnegative quantity with at most six decimal places")
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok || positive && v.Sign() <= 0 {
		return nil, s, InspectionReportsInvalid(f, "Cleared quantity must be positive")
	}
	return v, s, nil
}
func (s InspectionReportsSource) Guard() error {
	if !s.Available || len(s.Blockers) > 0 {
		return InspectionReportsInvalid("source", "Resolve project, package and current cleared-inspection blockers first")
	}
	for _, id := range []string{s.ProjectID, s.PackageID, s.InspectionID, s.OrderID} {
		if e := InspectionReportsID(id); e != nil {
			return e
		}
	}
	if s.InspectionVersion < 1 || s.InspectionRevision < 1 || s.OrderVersion < 2 || !InspectionReportsDate(s.ActualDate) || s.Number == "" || s.SupplierName == "" || s.Description == "" || s.UOM == "" || s.QAPReference == "" {
		return InspectionReportsInvalid("source", "Use the actual current inspection clearance and released order")
	}
	_, _, e := inspectionReportsQuantity("source", s.Quantity, true)
	return e
}
func InspectionReportsAccessGuard(a Principal, c InspectionReportsAccess, write bool) error {
	if !a.Active || a.ID == "" {
		return InspectionReportsUnauthorized
	}
	if !c.Read || write && (!c.Write || !c.Work) {
		return InspectionReportsForbidden
	}
	return nil
}
func InspectionReportsValidate(q InspectionReportsCommand) (InspectionReportsCommand, error) {
	if q.Version < 0 {
		return q, InspectionReportsInvalid("version", "Use the current nonnegative report version")
	}
	var e error
	q.Reason, e = InspectionReportsText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	switch q.Action {
	case "record":
		if q.Report == nil {
			return q, InspectionReportsInvalid("report", "Supply the report details")
		}
	case "revise":
		if q.Report != nil {
			return q, InspectionReportsInvalid("report", "Revision carries only its version and reason")
		}
	default:
		return q, InspectionReportsInvalid("action", "Choose record or revise")
	}
	return q, nil
}
func inspectionReportsReport(q InspectionReportsReportInput, s InspectionReportsSource, today string) (InspectionReportsReport, error) {
	out := InspectionReportsReport{Status: q.Status, IRNumber: q.IRNumber, IRNNumber: q.IRNNumber, IssueDate: q.IssueDate, Authority: q.Authority, Quantity: q.Quantity, UOM: s.UOM, DocumentReference: q.DocumentReference}
	if q.Status != "pending" && q.Status != "issued" {
		return out, InspectionReportsInvalid("status", "Choose pending or issued")
	}
	for _, v := range []struct {
		f        string
		p        *string
		n        int
		required bool
	}{{"ir_number", &out.IRNumber, 200, q.Status == "issued"}, {"irn_number", &out.IRNNumber, 200, q.Status == "issued"}, {"issuing_authority", &out.Authority, 300, false}, {"file_name", &out.DocumentReference, 500, false}} {
		var e error
		*v.p, e = InspectionReportsText(v.f, *v.p, v.n, v.required)
		if e != nil {
			return out, e
		}
	}
	if out.IssueDate != "" && (!InspectionReportsDate(out.IssueDate) || out.IssueDate < s.ActualDate || out.IssueDate > today) || out.Status == "issued" && out.IssueDate == "" {
		return out, InspectionReportsInvalid("issue_date", "Use the actual issue date on or after inspection and not in the future")
	}
	cleared, text, e := inspectionReportsQuantity("cleared_quantity", out.Quantity, out.Status == "issued")
	if e != nil {
		return out, e
	}
	out.Quantity = text
	ordered, _, e := inspectionReportsQuantity("source", s.Quantity, true)
	if e != nil {
		return out, e
	}
	if cleared.Cmp(ordered) > 0 {
		return out, InspectionReportsInvalid("cleared_quantity", "Cleared quantity cannot exceed this order's material quantity")
	}
	return out, nil
}
func InspectionReportsCan(r *InspectionReportsRecord, a Principal, c InspectionReportsAccess, action string, s InspectionReportsSource) error {
	if e := InspectionReportsAccessGuard(a, c, true); e != nil {
		return e
	}
	if e := s.Guard(); e != nil {
		return e
	}
	if action != "record" && action != "revise" {
		return InspectionReportsInvalid("action", "Choose record or revise")
	}
	if r == nil {
		if action != "record" {
			return InspectionReportsInvalid("report", "Record the report before starting a revision")
		}
		return nil
	}
	if !r.Active || r.ProjectID != s.ProjectID || r.PackageID != s.PackageID {
		return InspectionReportsMissing
	}
	if action == "record" && (r.Source.InspectionReportsSourceFacts != s.InspectionReportsSourceFacts || r.Report.Status == "issued") {
		return InspectionReportsStale
	}
	if action == "revise" && r.Report.Status != "issued" && r.Source.InspectionReportsSourceFacts == s.InspectionReportsSourceFacts {
		return InspectionReportsInvalid("revision", "Edit the current pending report instead")
	}
	return nil
}
func InspectionReportsApply(base *InspectionReportsRecord, a Principal, c InspectionReportsAccess, q InspectionReportsCommand, s InspectionReportsSource, now InspectionReportsTimestamp, id string) (InspectionReportsRecord, error) {
	empty := InspectionReportsRecord{}
	q, e := InspectionReportsValidate(q)
	if e != nil {
		return empty, e
	}
	if e = InspectionReportsCan(base, a, c, q.Action, s); e != nil {
		return empty, e
	}
	out := InspectionReportsRecord{ID: id, ProjectID: s.ProjectID, PackageID: s.PackageID, Revision: 1, Source: s, Active: true, CreatedAt: now, CreatedBy: a.ID}
	if base != nil {
		if q.Version != base.Version {
			return empty, InspectionReportsStale
		}
		out = InspectionReportsCopy(*base)
	} else if q.Version != 0 {
		return empty, InspectionReportsStale
	}
	for _, id := range []string{out.ID, a.ID} {
		if e = InspectionReportsID(id); e != nil {
			return empty, e
		}
	}
	if q.Action == "revise" {
		out.Revision++
		out.Source = s
		out.Report = InspectionReportsReport{Status: "pending", Quantity: "0", UOM: s.UOM}
	} else {
		out.Report, e = inspectionReportsReport(*q.Report, s, InspectionReportsToday(now))
		if e != nil {
			return empty, e
		}
	}
	out.Version = q.Version + 1
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}
func InspectionReportsIssuedFact(r *InspectionReportsRecord, s InspectionReportsSource) (*InspectionReportsRecord, error) {
	if e := s.Guard(); e != nil {
		return nil, e
	}
	if r == nil || !r.Active {
		return nil, InspectionReportsMissing
	}
	if r.ProjectID != s.ProjectID || r.PackageID != s.PackageID || r.Source.InspectionReportsSourceFacts != s.InspectionReportsSourceFacts {
		return nil, InspectionReportsStale
	}
	if r.Version < 1 || r.Revision < 1 || r.Report.Status != "issued" || r.Report.UOM != s.UOM {
		return nil, InspectionReportsInvalid("report", "Current issued IR / IRN is required")
	}
	q := r.Report
	if _, e := inspectionReportsReport(InspectionReportsReportInput{Status: q.Status, IRNumber: q.IRNumber, IRNNumber: q.IRNNumber, IssueDate: q.IssueDate, Authority: q.Authority, Quantity: q.Quantity, DocumentReference: q.DocumentReference}, s, InspectionReportsToday(r.UpdatedAt)); e != nil {
		return nil, e
	}
	out := InspectionReportsCopy(*r)
	return &out, nil
}
