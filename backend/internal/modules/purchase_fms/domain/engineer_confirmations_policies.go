package domain

import (
	"math/big"
	"regexp"
	"strings"
)

var engineerConfirmationsDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$`)

func engineerConfirmationsQuantity(f, s string, positive bool) (*big.Rat, string, error) {
	s = strings.TrimSpace(s)
	if s == "" && !positive {
		s = "0"
	}
	if !engineerConfirmationsDecimal.MatchString(s) {
		return nil, s, EngineerConfirmationsInvalid(f, "Use an exact nonnegative quantity with at most six decimal places")
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok || positive && v.Sign() <= 0 {
		return nil, s, EngineerConfirmationsInvalid(f, "Quantity must be positive")
	}
	return v, s, nil
}
func (s EngineerConfirmationsSource) Guard() error {
	if !s.Available || len(s.Blockers) > 0 {
		return EngineerConfirmationsInvalid("source", "Resolve current quality, IRN and project/package blockers first")
	}
	for _, id := range []string{s.ProjectID, s.PackageID, s.GRNID, s.ReceiptID, s.DispatchID, s.ClearanceID, s.QualityID, s.ReportID, s.OrderID, s.InspectionID} {
		if e := EngineerConfirmationsID(id); e != nil {
			return e
		}
	}
	if s.GRNVersion < 1 || s.GRNRevision < 1 || s.GRNNumber == "" || !EngineerConfirmationsDate(s.GRNDate) || s.GRNDate < s.ArrivalDate || s.ReceiptVersion < 1 || s.ReceiptRevision < 1 || !EngineerConfirmationsDate(s.ArrivalDate) || s.ArrivalDate < s.DispatchDate || s.ReceivingOfficer == "" || s.PackagesReceived < 0 || s.DispatchVersion < 1 || s.DispatchRevision < 1 || !EngineerConfirmationsDate(s.DispatchDate) || s.DispatchDate < s.ClearanceDate || s.LRNumber == "" || s.Transporter == "" || s.ClearanceVersion < 1 || s.ClearanceRevision < 1 || s.Reference == "" || !EngineerConfirmationsDate(s.ClearanceDate) || s.ClearanceDate < s.IssueDate || s.QualityVersion < 1 || s.QualityRevision < 1 || s.ReportVersion < 1 || s.ReportRevision < 1 || s.InspectionVersion < 1 || s.OrderVersion < 2 || !EngineerConfirmationsDate(s.IssueDate) || s.IRNumber == "" || s.IRNNumber == "" || s.Number == "" || s.SupplierName == "" || s.Description == "" || s.UOM == "" {
		return EngineerConfirmationsInvalid("source", "Use the actual current issued GRN and its clearance evidence")
	}
	allowance, _, e := engineerConfirmationsQuantity("source", s.Quantity, true)
	if e != nil {
		return e
	}
	received, _, e := engineerConfirmationsQuantity("source", s.Received, true)
	if e != nil {
		return e
	}
	accepted, _, e := engineerConfirmationsQuantity("source", s.Accepted, false)
	if e != nil {
		return e
	}
	rejected, _, e := engineerConfirmationsQuantity("source", s.Rejected, false)
	if e != nil {
		return e
	}
	if received.Cmp(allowance) > 0 || new(big.Rat).Add(accepted, rejected).Cmp(received) > 0 {
		return EngineerConfirmationsInvalid("source", "Use the actual current issued GRN breakdown")
	}
	return nil
}
func EngineerConfirmationsAccessGuard(a Principal, c EngineerConfirmationsAccess, write bool) error {
	if !a.Active || a.ID == "" {
		return EngineerConfirmationsUnauthorized
	}
	if !c.Read || write && (!c.Write || !c.Work) {
		return EngineerConfirmationsForbidden
	}
	return nil
}
func EngineerConfirmationsValidate(q EngineerConfirmationsCommand) (EngineerConfirmationsCommand, error) {
	if q.Version < 0 {
		return q, EngineerConfirmationsInvalid("version", "Use the current nonnegative confirmation version")
	}
	var e error
	q.Reason, e = EngineerConfirmationsText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	switch q.Action {
	case "record":
		if q.Confirmation == nil {
			return q, EngineerConfirmationsInvalid("confirmation", "Supply the confirmation details")
		}
	case "revise":
		if q.Confirmation != nil {
			return q, EngineerConfirmationsInvalid("confirmation", "Revision carries only its version and reason")
		}
	default:
		return q, EngineerConfirmationsInvalid("action", "Choose record or revise")
	}
	return q, nil
}
func engineerConfirmationsConfirmation(q EngineerConfirmationsConfirmationInput, s EngineerConfirmationsSource, today string) (EngineerConfirmationsConfirmation, error) {
	out := q
	if out.Status != "pending" && out.Status != "confirmed" {
		return out, EngineerConfirmationsInvalid("status", "Choose pending or confirmed")
	}
	for _, v := range []struct {
		f        string
		p        *string
		n        int
		required bool
	}{
		{"engineer_name", &out.EngineerName, 300, out.Status == "confirmed"}, {"designation", &out.Designation, 200, false}, {"mbook_ref", &out.MBookReference, 200, false}, {"work_package_ref", &out.WorkPackageReference, 200, false}, {"remarks", &out.Remarks, 4000, false},
	} {
		var e error
		*v.p, e = EngineerConfirmationsText(v.f, *v.p, v.n, v.required)
		if e != nil {
			return out, e
		}
	}
	if out.Date != "" && (!EngineerConfirmationsDate(out.Date) || out.Date < s.GRNDate || out.Date > today) || out.Status == "confirmed" && out.Date == "" {
		return out, EngineerConfirmationsInvalid("confirmation_date", "Use the actual confirmation date on or after GRN issuance and not in the future")
	}
	return out, nil
}
func EngineerConfirmationsCan(r *EngineerConfirmationsRecord, a Principal, c EngineerConfirmationsAccess, action string, s EngineerConfirmationsSource) error {
	if e := EngineerConfirmationsAccessGuard(a, c, true); e != nil {
		return e
	}
	if e := s.Guard(); e != nil {
		return e
	}
	if action != "record" && action != "revise" {
		return EngineerConfirmationsInvalid("action", "Choose record or revise")
	}
	if r == nil {
		if action != "record" {
			return EngineerConfirmationsInvalid("confirmation", "Record the confirmation before starting a revision")
		}
		return nil
	}
	if !r.Active || r.ProjectID != s.ProjectID || r.PackageID != s.PackageID {
		return EngineerConfirmationsMissing
	}
	if action == "record" && (r.Source.EngineerConfirmationsSourceFacts != s.EngineerConfirmationsSourceFacts || r.Confirmation.Status == "confirmed") {
		return EngineerConfirmationsStale
	}
	if action == "revise" && r.Confirmation.Status != "confirmed" && r.Source.EngineerConfirmationsSourceFacts == s.EngineerConfirmationsSourceFacts {
		return EngineerConfirmationsInvalid("revision", "Edit the current pending Confirmation instead")
	}
	return nil
}
func EngineerConfirmationsApply(base *EngineerConfirmationsRecord, a Principal, c EngineerConfirmationsAccess, q EngineerConfirmationsCommand, s EngineerConfirmationsSource, now EngineerConfirmationsTimestamp, id string) (EngineerConfirmationsRecord, error) {
	empty := EngineerConfirmationsRecord{}
	q, e := EngineerConfirmationsValidate(q)
	if e != nil {
		return empty, e
	}
	if e = EngineerConfirmationsCan(base, a, c, q.Action, s); e != nil {
		return empty, e
	}
	out := EngineerConfirmationsRecord{ID: id, ProjectID: s.ProjectID, PackageID: s.PackageID, Revision: 1, Source: s, Active: true, CreatedAt: now, CreatedBy: a.ID}
	if base != nil {
		if q.Version != base.Version {
			return empty, EngineerConfirmationsStale
		}
		out = EngineerConfirmationsCopy(*base)
	} else if q.Version != 0 {
		return empty, EngineerConfirmationsStale
	}
	for _, id := range []string{out.ID, a.ID} {
		if e = EngineerConfirmationsID(id); e != nil {
			return empty, e
		}
	}
	if q.Action == "revise" {
		out.Revision++
		out.Source = s
		out.Confirmation = EngineerConfirmationsConfirmation{Status: "pending"}
	} else {
		out.Confirmation, e = engineerConfirmationsConfirmation(*q.Confirmation, s, EngineerConfirmationsToday(now))
		if e != nil {
			return empty, e
		}
	}
	out.Version = q.Version + 1
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}
func EngineerConfirmationsConfirmedFact(r *EngineerConfirmationsRecord, s EngineerConfirmationsSource) (*EngineerConfirmationsRecord, error) {
	if e := s.Guard(); e != nil {
		return nil, e
	}
	if r == nil || !r.Active {
		return nil, EngineerConfirmationsMissing
	}
	if r.ProjectID != s.ProjectID || r.PackageID != s.PackageID || r.Source.EngineerConfirmationsSourceFacts != s.EngineerConfirmationsSourceFacts {
		return nil, EngineerConfirmationsStale
	}
	if r.Version < 1 || r.Revision < 1 || r.Confirmation.Status != "confirmed" {
		return nil, EngineerConfirmationsInvalid("confirmation", "Current issued Confirmation is required")
	}
	if _, e := engineerConfirmationsConfirmation(r.Confirmation, s, EngineerConfirmationsToday(r.UpdatedAt)); e != nil {
		return nil, e
	}
	out := EngineerConfirmationsCopy(*r)
	return &out, nil
}
