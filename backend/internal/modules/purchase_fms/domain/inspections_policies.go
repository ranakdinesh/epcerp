package domain

import (
	"fmt"
	"strings"
)

func (s InspectionsSource) Guard() error {
	if !s.Available || len(s.Blockers) > 0 {
		return InspectionsInvalid("source", "Resolve project, released PO, acceptance and approved QAP blockers first")
	}
	for _, id := range []string{s.ProjectID, s.PackageID, s.OrderID, s.SupplierID, s.AcceptanceID, s.TechnicalID, s.QAPFileID, s.SignedApprovalFileID} {
		if e := InspectionsID(id); e != nil {
			return e
		}
	}
	if s.OrderVersion < 2 || s.AcceptanceVersion < 1 || s.TechnicalVersion < 1 || s.TechnicalRevision < 1 || s.Number == "" || !InspectionsDate(s.Date) || !InspectionsDate(s.QAPDate) || s.QAPReference == "" || s.SupplierName == "" || s.Description == "" || s.Quantity == "" || s.UOM == "" {
		return InspectionsInvalid("source", "Use the actual released PO, unconditional acceptance and approved QAP")
	}
	return nil
}
func InspectionsAccessGuard(a Principal, c InspectionsAccess, write bool) error {
	if !a.Active || a.ID == "" {
		return InspectionsUnauthorized
	}
	if !c.Read || write && (!c.Write || !c.Work) {
		return InspectionsForbidden
	}
	return nil
}
func InspectionsValidate(q InspectionsCommand) (InspectionsCommand, error) {
	if q.Version < 0 {
		return q, InspectionsInvalid("version", "Use the current nonnegative inspection version")
	}
	var e error
	q.Reason, e = InspectionsText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	if q.Action == "record" {
		if q.Inspection == nil {
			return q, InspectionsInvalid("inspection", "Supply the inspection details")
		}
	} else if q.Action == "revise" {
		if q.Inspection != nil {
			return q, InspectionsInvalid("inspection", "Revision carries only its version and reason")
		}
	} else {
		return q, InspectionsInvalid("action", "Choose record or revise")
	}
	return q, nil
}
func InspectionsFinal(status string) bool { return status == "cleared" || status == "rejected" }
func inspectionsInspection(q InspectionsInspectionInput, s InspectionsSource, today string) (InspectionsInspection, error) {
	out := InspectionsInspection{Status: q.Status, CallReference: q.CallReference, CallDate: q.CallDate, ScheduledDate: q.ScheduledDate, ActualDate: q.ActualDate, Agency: q.Agency, Inspector: q.Inspector, Factory: q.Factory, Points: []InspectionsWitnessPoint{}, NoPointsReason: q.NoPointsReason, Findings: q.Findings}
	if out.Status != "not_scheduled" && out.Status != "call_received" && out.Status != "scheduled" && !InspectionsFinal(out.Status) {
		return out, InspectionsInvalid("status", "Choose a declared inspection state")
	}
	for _, v := range []struct {
		f        string
		p        *string
		n        int
		required bool
	}{
		{"call_notice_ref", &out.CallReference, 200, out.Status == "call_received"},
		{"factory_location", &out.Factory, 1000, out.Status == "scheduled" || InspectionsFinal(out.Status)},
		{"inspector_name", &out.Inspector, 200, InspectionsFinal(out.Status)},
		{"findings", &out.Findings, 8000, InspectionsFinal(out.Status)},
		{"no_witness_points_reason", &out.NoPointsReason, 2000, out.Status == "cleared" && len(q.Points) == 0},
	} {
		var e error
		*v.p, e = InspectionsText(v.f, *v.p, v.n, v.required)
		if e != nil {
			return out, e
		}
	}
	if out.Agency != "" && out.Agency != "client_consultant" && out.Agency != "tpi" && out.Agency != "internal_qc" || InspectionsFinal(out.Status) && out.Agency == "" {
		return out, InspectionsInvalid("inspection_agency", "Choose Client / Consultant, Third Party Inspection or Internal Quality Assurance")
	}
	for _, v := range []struct {
		f, s             string
		future, required bool
	}{
		{"call_date", out.CallDate, false, out.Status == "call_received"},
		{"scheduled_date", out.ScheduledDate, true, false},
		{"actual_date", out.ActualDate, false, InspectionsFinal(out.Status)},
	} {
		if v.s != "" && (!InspectionsDate(v.s) || v.s < s.Date || !v.future && v.s > today) || v.required && v.s == "" {
			return out, InspectionsInvalid(v.f, "Use a truthful inspection date on or after PO issue; actual/call dates cannot be future")
		}
	}
	if out.Status == "scheduled" && out.ScheduledDate == "" && out.ActualDate == "" {
		return out, InspectionsInvalid("scheduled_date", "Provide a scheduled or actual inspection date")
	}
	for _, v := range []struct{ f, s string }{{"scheduled_date", out.ScheduledDate}, {"actual_date", out.ActualDate}} {
		if v.s != "" && out.CallDate != "" && v.s < out.CallDate {
			return out, InspectionsInvalid(v.f, "Inspection cannot precede the recorded vendor call")
		}
	}
	if out.ActualDate != "" && out.ActualDate < s.QAPDate {
		return out, InspectionsInvalid("actual_date", "Inspect against the QAP approved before the actual visit")
	}
	if len(q.Points) > 100 {
		return out, InspectionsInvalid("witness_points", "Keep an inspection checklist within 100 points")
	}
	codes := map[string]bool{}
	for i, p := range q.Points {
		path := fmt.Sprintf("witness_points.%d", i)
		var e error
		p.Code, e = InspectionsText(path+".code", p.Code, 60, true)
		if e != nil {
			return out, e
		}
		p.Title, e = InspectionsText(path+".title", p.Title, 1000, true)
		if e != nil {
			return out, e
		}
		p.Remarks, e = InspectionsText(path+".remarks", p.Remarks, 2000, false)
		if e != nil {
			return out, e
		}
		key := strings.ToLower(p.Code)
		if codes[key] {
			return out, InspectionsInvalid(path+".code", "Use distinct witness point codes")
		}
		codes[key] = true
		if p.Mandatory == nil || p.Cleared == nil {
			return out, InspectionsInvalid(path, "Declare mandatory and cleared flags explicitly")
		}
		if out.Status == "cleared" && *p.Mandatory && !*p.Cleared {
			return out, InspectionsInvalid(path+".is_cleared", "Clear every mandatory QAP witness point before inspection clearance")
		}
		out.Points = append(out.Points, InspectionsWitnessPoint{Code: p.Code, Title: p.Title, Mandatory: *p.Mandatory, Cleared: *p.Cleared, Remarks: p.Remarks})
	}
	if len(out.Points) > 0 && out.NoPointsReason != "" {
		return out, InspectionsInvalid("no_witness_points_reason", "A no-points reason belongs only to an empty QAP checklist")
	}
	return out, nil
}
func InspectionsCan(r *InspectionsRecord, a Principal, c InspectionsAccess, action string, s InspectionsSource) error {
	if e := InspectionsAccessGuard(a, c, true); e != nil {
		return e
	}
	if e := s.Guard(); e != nil {
		return e
	}
	if action != "record" && action != "revise" {
		return InspectionsInvalid("action", "Choose record or revise")
	}
	if r == nil {
		if action != "record" {
			return InspectionsInvalid("inspection", "Record the inspection before starting a revision")
		}
		return nil
	}
	if !r.Active || r.ProjectID != s.ProjectID || r.PackageID != s.PackageID {
		return InspectionsMissing
	}
	if action == "record" && (r.Source.InspectionsSourceFacts != s.InspectionsSourceFacts || InspectionsFinal(r.Inspection.Status)) {
		return InspectionsStale
	}
	if action == "revise" && !InspectionsFinal(r.Inspection.Status) && r.Source.InspectionsSourceFacts == s.InspectionsSourceFacts {
		return InspectionsInvalid("revision", "Edit the current non-final inspection instead")
	}
	return nil
}
func InspectionsApply(base *InspectionsRecord, a Principal, c InspectionsAccess, q InspectionsCommand, s InspectionsSource, now InspectionsTimestamp, id string) (InspectionsRecord, error) {
	empty := InspectionsRecord{}
	q, e := InspectionsValidate(q)
	if e != nil {
		return empty, e
	}
	if e = InspectionsCan(base, a, c, q.Action, s); e != nil {
		return empty, e
	}
	out := InspectionsRecord{ID: id, ProjectID: s.ProjectID, PackageID: s.PackageID, Revision: 1, Source: s, Active: true, CreatedAt: now, CreatedBy: a.ID}
	if base != nil {
		if q.Version != base.Version {
			return empty, InspectionsStale
		}
		out = InspectionsCopy(*base)
	} else if q.Version != 0 {
		return empty, InspectionsStale
	}
	for _, id := range []string{out.ID, a.ID} {
		if e = InspectionsID(id); e != nil {
			return empty, e
		}
	}
	if q.Action == "revise" {
		out.Revision++
		out.Source = s
		out.Inspection = InspectionsInspection{Status: "not_scheduled", Points: []InspectionsWitnessPoint{}}
	} else {
		out.Inspection, e = inspectionsInspection(*q.Inspection, s, InspectionsToday(now))
		if e != nil {
			return empty, e
		}
	}
	out.Version = q.Version + 1
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}

// Clearance is a current fact; it grants no actor authority or dispatch approval.
func InspectionsClearedFact(r *InspectionsRecord, s InspectionsSource) (*InspectionsRecord, error) {
	if e := s.Guard(); e != nil {
		return nil, e
	}
	if r == nil || !r.Active {
		return nil, InspectionsMissing
	}
	if r.Source.InspectionsSourceFacts != s.InspectionsSourceFacts || r.ProjectID != s.ProjectID || r.PackageID != s.PackageID {
		return nil, InspectionsStale
	}
	if r.Version < 1 || r.Revision < 1 || r.Inspection.Status != "cleared" {
		return nil, InspectionsInvalid("inspection", "Current material inspection clearance is required")
	}
	in := InspectionsInspectionInput{Status: r.Inspection.Status, CallReference: r.Inspection.CallReference, CallDate: r.Inspection.CallDate, ScheduledDate: r.Inspection.ScheduledDate, ActualDate: r.Inspection.ActualDate, Agency: r.Inspection.Agency, Inspector: r.Inspection.Inspector, Factory: r.Inspection.Factory, NoPointsReason: r.Inspection.NoPointsReason, Findings: r.Inspection.Findings}
	for _, p := range r.Inspection.Points {
		mandatory, cleared := p.Mandatory, p.Cleared
		in.Points = append(in.Points, InspectionsWitnessInput{Code: p.Code, Title: p.Title, Mandatory: &mandatory, Cleared: &cleared, Remarks: p.Remarks})
	}
	if _, e := inspectionsInspection(in, s, InspectionsToday(r.UpdatedAt)); e != nil {
		return nil, e
	}
	out := InspectionsCopy(*r)
	return &out, nil
}
