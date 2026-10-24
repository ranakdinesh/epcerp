package domain

import (
	"math/big"
	"regexp"
	"strings"
)

var clientClearanceDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$`)

func clientClearanceQuantity(f, s string, positive bool) (*big.Rat, string, error) {
	s = strings.TrimSpace(s)
	if s == "" && !positive {
		s = "0"
	}
	if !clientClearanceDecimal.MatchString(s) {
		return nil, s, ClientClearanceInvalid(f, "Use an exact nonnegative quantity with at most six decimal places")
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok || positive && v.Sign() <= 0 {
		return nil, s, ClientClearanceInvalid(f, "Cleared quantity must be positive")
	}
	return v, s, nil
}
func (s ClientClearanceSource) Guard() error {
	if !s.Available || len(s.Blockers) > 0 {
		return ClientClearanceInvalid("source", "Resolve current quality, IRN and project/package blockers first")
	}
	for _, id := range []string{s.ProjectID, s.PackageID, s.QualityID, s.ReportID, s.OrderID, s.InspectionID} {
		if e := ClientClearanceID(id); e != nil {
			return e
		}
	}
	if s.QualityVersion < 1 || s.QualityRevision < 1 || s.ReportVersion < 1 || s.ReportRevision < 1 || s.InspectionVersion < 1 || s.OrderVersion < 2 || !ClientClearanceDate(s.IssueDate) || s.IRNumber == "" || s.IRNNumber == "" || s.Number == "" || s.SupplierName == "" || s.Description == "" || s.UOM == "" {
		return ClientClearanceInvalid("source", "Use the actual current verified quality and issued IRN")
	}
	_, _, e := clientClearanceQuantity("source", s.Quantity, true)
	return e
}
func ClientClearanceAccessGuard(a Principal, c ClientClearanceAccess, write bool) error {
	if !a.Active || a.ID == "" {
		return ClientClearanceUnauthorized
	}
	if !c.Read || write && (!c.Write || !c.Work) {
		return ClientClearanceForbidden
	}
	return nil
}
func ClientClearanceValidate(q ClientClearanceCommand) (ClientClearanceCommand, error) {
	if q.Version < 0 {
		return q, ClientClearanceInvalid("version", "Use the current nonnegative clearance version")
	}
	var e error
	q.Reason, e = ClientClearanceText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	switch q.Action {
	case "record":
		if q.Clearance == nil {
			return q, ClientClearanceInvalid("clearance", "Supply the clearance details")
		}
	case "revise":
		if q.Clearance != nil {
			return q, ClientClearanceInvalid("clearance", "Revision carries only its version and reason")
		}
	default:
		return q, ClientClearanceInvalid("action", "Choose record or revise")
	}
	return q, nil
}
func clientClearanceClearance(q ClientClearanceClearanceInput, s ClientClearanceSource, today string) (ClientClearanceClearance, error) {
	out := q
	if q.Status != "pending" && q.Status != "cleared" {
		return out, ClientClearanceInvalid("status", "Choose pending or cleared")
	}
	for _, v := range []struct {
		f        string
		p        *string
		n        int
		required bool
	}{{"mdcc_ref", &out.Reference, 200, q.Status == "cleared"}, {"cleared_by_authority", &out.Authority, 300, false}, {"remarks", &out.Remarks, 4000, false}, {"file_name", &out.DocumentReference, 500, false}} {
		var e error
		*v.p, e = ClientClearanceText(v.f, *v.p, v.n, v.required)
		if e != nil {
			return out, e
		}
	}
	if out.Date != "" && (!ClientClearanceDate(out.Date) || out.Date < s.IssueDate || out.Date > today) || out.Status == "cleared" && out.Date == "" {
		return out, ClientClearanceInvalid("clearance_date", "Use the actual clearance date on or after issued IRN and not in the future")
	}
	return out, nil
}
func ClientClearanceCan(r *ClientClearanceRecord, a Principal, c ClientClearanceAccess, action string, s ClientClearanceSource) error {
	if e := ClientClearanceAccessGuard(a, c, true); e != nil {
		return e
	}
	if e := s.Guard(); e != nil {
		return e
	}
	if action != "record" && action != "revise" {
		return ClientClearanceInvalid("action", "Choose record or revise")
	}
	if r == nil {
		if action != "record" {
			return ClientClearanceInvalid("clearance", "Record the clearance before starting a revision")
		}
		return nil
	}
	if !r.Active || r.ProjectID != s.ProjectID || r.PackageID != s.PackageID {
		return ClientClearanceMissing
	}
	if action == "record" && (r.Source.ClientClearanceSourceFacts != s.ClientClearanceSourceFacts || r.Clearance.Status == "cleared") {
		return ClientClearanceStale
	}
	if action == "revise" && r.Clearance.Status != "cleared" && r.Source.ClientClearanceSourceFacts == s.ClientClearanceSourceFacts {
		return ClientClearanceInvalid("revision", "Edit the current pending clearance instead")
	}
	return nil
}
func ClientClearanceApply(base *ClientClearanceRecord, a Principal, c ClientClearanceAccess, q ClientClearanceCommand, s ClientClearanceSource, now ClientClearanceTimestamp, id string) (ClientClearanceRecord, error) {
	empty := ClientClearanceRecord{}
	q, e := ClientClearanceValidate(q)
	if e != nil {
		return empty, e
	}
	if e = ClientClearanceCan(base, a, c, q.Action, s); e != nil {
		return empty, e
	}
	out := ClientClearanceRecord{ID: id, ProjectID: s.ProjectID, PackageID: s.PackageID, Revision: 1, Source: s, Active: true, CreatedAt: now, CreatedBy: a.ID}
	if base != nil {
		if q.Version != base.Version {
			return empty, ClientClearanceStale
		}
		out = ClientClearanceCopy(*base)
	} else if q.Version != 0 {
		return empty, ClientClearanceStale
	}
	for _, id := range []string{out.ID, a.ID} {
		if e = ClientClearanceID(id); e != nil {
			return empty, e
		}
	}
	if q.Action == "revise" {
		out.Revision++
		out.Source = s
		out.Clearance = ClientClearanceClearance{Status: "pending"}
	} else {
		out.Clearance, e = clientClearanceClearance(*q.Clearance, s, ClientClearanceToday(now))
		if e != nil {
			return empty, e
		}
	}
	out.Version = q.Version + 1
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}
func ClientClearanceClearedFact(r *ClientClearanceRecord, s ClientClearanceSource) (*ClientClearanceRecord, error) {
	if e := s.Guard(); e != nil {
		return nil, e
	}
	if r == nil || !r.Active {
		return nil, ClientClearanceMissing
	}
	if r.ProjectID != s.ProjectID || r.PackageID != s.PackageID || r.Source.ClientClearanceSourceFacts != s.ClientClearanceSourceFacts {
		return nil, ClientClearanceStale
	}
	if r.Version < 1 || r.Revision < 1 || r.Clearance.Status != "cleared" {
		return nil, ClientClearanceInvalid("clearance", "Current issued Client dispatch clearance is required")
	}
	if _, e := clientClearanceClearance(r.Clearance, s, ClientClearanceToday(r.UpdatedAt)); e != nil {
		return nil, e
	}
	out := ClientClearanceCopy(*r)
	return &out, nil
}
