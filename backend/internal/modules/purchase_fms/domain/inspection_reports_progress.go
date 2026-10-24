package domain

// Reporting issuance never grants authority for later client clearance or dispatch.
type InspectionReportsProgress struct {
	ID                string
	Version, Revision int32
	State, Details    string
	Current           bool
	Snapshot          *InspectionReportsRecord `json:"-"`
}

func InspectionReportsProgressFact(r *InspectionReportsRecord, expected InspectionReportsSourceFacts, upstreamCurrent bool) InspectionReportsProgress {
	out := InspectionReportsProgress{State: "not_started"}
	if r == nil || !r.Active {
		return out
	}
	out.ID, out.Version, out.Revision = r.ID, r.Version, r.Revision
	source := InspectionReportsSource{InspectionReportsSourceFacts: expected, Available: true}
	if !upstreamCurrent || source.Guard() != nil || r.ProjectID != expected.ProjectID || r.PackageID != expected.PackageID || r.Source.InspectionReportsSourceFacts != expected {
		out.State, out.Details = "blocked", "Report evidence does not match the current cleared inspection"
		return out
	}
	out.Current = true
	if r.Report.Status == "pending" {
		return out
	}
	snapshot, err := InspectionReportsIssuedFact(r, source)
	if err != nil {
		out.State, out.Details = "blocked", "Stored IR / IRN issuance evidence is incomplete"
		return out
	}
	out.State, out.Details, out.Snapshot = "completed", r.Report.IRNNumber, snapshot
	return out
}
