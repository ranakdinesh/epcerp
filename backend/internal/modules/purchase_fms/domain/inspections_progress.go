package domain

// Progress reports inspection work; it grants no clearance to downstream commands.
type InspectionsProgress struct {
	ID                string
	Version, Revision int32
	State, Details    string
	Current           bool
	Snapshot          *InspectionsRecord `json:"-"`
}

func InspectionsProgressFact(r *InspectionsRecord, expected InspectionsSourceFacts, upstreamCurrent bool) InspectionsProgress {
	out := InspectionsProgress{State: "not_started"}
	if r == nil || !r.Active {
		return out
	}
	out.ID, out.Version, out.Revision = r.ID, r.Version, r.Revision
	source := InspectionsSource{InspectionsSourceFacts: expected, Available: true}
	if !upstreamCurrent || source.Guard() != nil || r.ProjectID != expected.ProjectID || r.PackageID != expected.PackageID || r.Source.InspectionsSourceFacts != expected {
		out.State, out.Details = "blocked", "Inspection evidence does not match the current PO, acceptance and approved technical kit"
		return out
	}
	out.Current = true
	switch r.Inspection.Status {
	case "not_scheduled":
		return out
	case "cleared":
		snapshot, err := InspectionsClearedFact(r, source)
		if err != nil {
			out.State, out.Details = "blocked", "Stored inspection clearance evidence is incomplete"
			return out
		}
		out.State, out.Details, out.Snapshot = "completed", r.Inspection.Findings, snapshot
	case "rejected":
		out.State, out.Details = "blocked", r.Inspection.Findings
	case "call_received", "scheduled":
		input := InspectionsInspectionInput{Status: r.Inspection.Status, CallReference: r.Inspection.CallReference, CallDate: r.Inspection.CallDate, ScheduledDate: r.Inspection.ScheduledDate, ActualDate: r.Inspection.ActualDate, Agency: r.Inspection.Agency, Inspector: r.Inspection.Inspector, Factory: r.Inspection.Factory, NoPointsReason: r.Inspection.NoPointsReason, Findings: r.Inspection.Findings}
		for _, p := range r.Inspection.Points {
			mandatory, cleared := p.Mandatory, p.Cleared
			input.Points = append(input.Points, InspectionsWitnessInput{Code: p.Code, Title: p.Title, Mandatory: &mandatory, Cleared: &cleared, Remarks: p.Remarks})
		}
		if _, err := inspectionsInspection(input, source, InspectionsToday(r.UpdatedAt)); err != nil {
			out.State, out.Details = "blocked", "Stored inspection scheduling evidence is incomplete"
			return out
		}
		out.State, out.Details = "in_progress", r.Inspection.CallReference
	default:
		out.State, out.Details = "blocked", "Stored inspection state is invalid"
	}
	return out
}
