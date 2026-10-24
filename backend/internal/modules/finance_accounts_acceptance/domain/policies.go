package domain

import (
	"math/big"
	"regexp"
	"strings"
)

var decimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$`)

func quantity(f, s string, positive bool) (*big.Rat, string, error) {
	s = strings.TrimSpace(s)
	if s == "" && !positive {
		s = "0"
	}
	if !decimal.MatchString(s) {
		return nil, s, Invalid(f, "Use an exact nonnegative quantity with at most six decimal places")
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok || positive && v.Sign() <= 0 {
		return nil, s, Invalid(f, "Quantity must be positive")
	}
	return v, s, nil
}
func (s Source) Guard() error {
	if !s.Available || len(s.Blockers) > 0 {
		return Invalid("source", "Resolve current quality, IRN and project/package blockers first")
	}
	for _, id := range []string{s.ProjectID, s.PackageID, s.EngineerID, s.GRNID, s.ReceiptID, s.DispatchID, s.ClearanceID, s.QualityID, s.ReportID, s.OrderID, s.InspectionID} {
		if e := ID(id); e != nil {
			return e
		}
	}
	if s.EngineerVersion < 1 || s.EngineerRevision < 1 || !Date(s.ConfirmationDate) || s.ConfirmationDate < s.GRNDate || s.EngineerName == "" || !regexp.MustCompile(`^[A-Z]{3}$`).MatchString(s.Currency) || s.GRNVersion < 1 || s.GRNRevision < 1 || s.GRNNumber == "" || !Date(s.GRNDate) || s.GRNDate < s.ArrivalDate || s.ReceiptVersion < 1 || s.ReceiptRevision < 1 || !Date(s.ArrivalDate) || s.ArrivalDate < s.DispatchDate || s.ReceivingOfficer == "" || s.PackagesReceived < 0 || s.DispatchVersion < 1 || s.DispatchRevision < 1 || !Date(s.DispatchDate) || s.DispatchDate < s.ClearanceDate || s.LRNumber == "" || s.Transporter == "" || s.ClearanceVersion < 1 || s.ClearanceRevision < 1 || s.Reference == "" || !Date(s.ClearanceDate) || s.ClearanceDate < s.IssueDate || s.QualityVersion < 1 || s.QualityRevision < 1 || s.ReportVersion < 1 || s.ReportRevision < 1 || s.InspectionVersion < 1 || s.OrderVersion < 2 || !Date(s.IssueDate) || s.IRNumber == "" || s.IRNNumber == "" || s.Number == "" || s.SupplierName == "" || s.Description == "" || s.UOM == "" {
		return Invalid("source", "Use the actual current issued GRN and its clearance evidence")
	}
	allowance, _, e := quantity("source", s.Quantity, true)
	if e != nil {
		return e
	}
	received, _, e := quantity("source", s.Received, true)
	if e != nil {
		return e
	}
	accepted, _, e := quantity("source", s.Accepted, false)
	if e != nil {
		return e
	}
	rejected, _, e := quantity("source", s.Rejected, false)
	if e != nil {
		return e
	}
	if received.Cmp(allowance) > 0 || new(big.Rat).Add(accepted, rejected).Cmp(received) > 0 {
		return Invalid("source", "Use the actual current issued GRN breakdown")
	}
	return nil
}
func AccessGuard(a Actor, c Access, write bool) error {
	if !a.Active || a.ID == "" {
		return Unauthorized
	}
	if !c.Read || write && (!c.Write || !c.Work) {
		return Forbidden
	}
	return nil
}
func Validate(q Command) (Command, error) {
	if q.Version < 0 {
		return q, Invalid("version", "Use the current nonnegative acceptance version")
	}
	var e error
	q.Reason, e = Text("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	switch q.Action {
	case "record":
		if q.Acceptance == nil {
			return q, Invalid("acceptance", "Supply the acceptance details")
		}
	case "revise":
		if q.Acceptance != nil {
			return q, Invalid("acceptance", "Revision carries only its version and reason")
		}
	default:
		return q, Invalid("action", "Choose record or revise")
	}
	return q, nil
}

var money = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,2})?$`)

func acceptance(q AcceptanceInput, s Source, today string) (Acceptance, error) {
	out := q
	if out.Status != "pending" && out.Status != "accepted" {
		return out, Invalid("status", "Choose pending or accepted")
	}
	for _, v := range []struct {
		f        string
		p        *string
		n        int
		required bool
	}{{"officer_name", &out.OfficerName, 300, out.Status == "accepted"}, {"voucher_ref", &out.VoucherReference, 200, false}, {"notes", &out.Notes, 4000, false}} {
		var e error
		*v.p, e = Text(v.f, *v.p, v.n, v.required)
		if e != nil {
			return out, e
		}
	}
	out.BookedAmount = strings.TrimSpace(out.BookedAmount)
	if out.BookedAmount == "" {
		out.BookedAmount = "0"
	}
	if !money.MatchString(out.BookedAmount) {
		return out, Invalid("booked_amount", "Use an exact nonnegative amount with at most two decimal places")
	}
	if out.Date != "" && (!Date(out.Date) || out.Date < s.ConfirmationDate || out.Date > today) || out.Status == "accepted" && out.Date == "" {
		return out, Invalid("acceptance_date", "Use actual acceptance date on or after engineer confirmation and not in the future")
	}
	return out, nil
}
func Can(r *Record, a Actor, c Access, action string, s Source) error {
	if e := AccessGuard(a, c, true); e != nil {
		return e
	}
	if e := s.Guard(); e != nil {
		return e
	}
	if action != "record" && action != "revise" {
		return Invalid("action", "Choose record or revise")
	}
	if r == nil {
		if action != "record" {
			return Invalid("acceptance", "Record the acceptance before starting a revision")
		}
		return nil
	}
	if !r.Active || r.ProjectID != s.ProjectID || r.PackageID != s.PackageID {
		return Missing
	}
	if action == "record" && (r.Source.SourceFacts != s.SourceFacts || r.Acceptance.Status == "accepted") {
		return Stale
	}
	if action == "revise" && r.Acceptance.Status != "accepted" && r.Source.SourceFacts == s.SourceFacts {
		return Invalid("revision", "Edit the current pending Acceptance instead")
	}
	return nil
}
func Apply(base *Record, a Actor, c Access, q Command, s Source, now Timestamp, id string) (Record, error) {
	empty := Record{}
	q, e := Validate(q)
	if e != nil {
		return empty, e
	}
	if e = Can(base, a, c, q.Action, s); e != nil {
		return empty, e
	}
	out := Record{ID: id, ProjectID: s.ProjectID, PackageID: s.PackageID, Revision: 1, Source: s, Active: true, CreatedAt: now, CreatedBy: a.ID}
	if base != nil {
		if q.Version != base.Version {
			return empty, Stale
		}
		out = Copy(*base)
	} else if q.Version != 0 {
		return empty, Stale
	}
	for _, id := range []string{out.ID, a.ID} {
		if e = ID(id); e != nil {
			return empty, e
		}
	}
	if q.Action == "revise" {
		out.Revision++
		out.Source = s
		out.Acceptance = Acceptance{Status: "pending", BookedAmount: "0"}
	} else {
		out.Acceptance, e = acceptance(*q.Acceptance, s, Today(now))
		if e != nil {
			return empty, e
		}
	}
	out.Version = q.Version + 1
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}
func AcceptedFact(r *Record, s Source) (*Record, error) {
	if e := s.Guard(); e != nil {
		return nil, e
	}
	if r == nil || !r.Active {
		return nil, Missing
	}
	if r.ProjectID != s.ProjectID || r.PackageID != s.PackageID || r.Source.SourceFacts != s.SourceFacts {
		return nil, Stale
	}
	if r.Version < 1 || r.Revision < 1 || r.Acceptance.Status != "accepted" {
		return nil, Invalid("acceptance", "Current issued Acceptance is required")
	}
	if _, e := acceptance(r.Acceptance, s, Today(r.UpdatedAt)); e != nil {
		return nil, e
	}
	out := Copy(*r)
	return &out, nil
}

// FinancialEvent is a completed Finance decision or an explicit return for correction.
func FinancialEvent(e Event) bool {
	return e.After.Acceptance.Status == "accepted" || e.Action == "revise" && e.Before != nil && e.Before.Acceptance.Status == "accepted"
}
func EventAction(e Event) string {
	if e.After.Acceptance.Status == "accepted" {
		return "approved"
	}
	if FinancialEvent(e) {
		return "returned"
	}
	return e.Action
}
