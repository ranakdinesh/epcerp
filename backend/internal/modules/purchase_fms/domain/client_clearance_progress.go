package domain

// Progress is recorded work, never a downstream execution authorization.
type ClientClearanceProgress struct {
	ID                string
	Version, Revision int32
	State, Details    string
	Current           bool
	Snapshot          *ClientClearanceRecord `json:"-"`
}

func ClientClearanceProgressFact(r *ClientClearanceRecord, expected ClientClearanceSourceFacts, upstreamCurrent bool) ClientClearanceProgress {
	out := ClientClearanceProgress{State: "not_started"}
	if r == nil || !r.Active {
		return out
	}
	out.ID, out.Version, out.Revision = r.ID, r.Version, r.Revision
	source := ClientClearanceSource{ClientClearanceSourceFacts: expected, Available: true}
	if !upstreamCurrent || source.Guard() != nil || r.ProjectID != expected.ProjectID || r.PackageID != expected.PackageID || r.Source.ClientClearanceSourceFacts != expected {
		out.State, out.Details = "blocked", "Recorded evidence does not match current upstream records"
		return out
	}
	out.Current = true
	if r.Clearance.Status == "pending" {
		return out
	}
	snapshot, err := ClientClearanceClearedFact(r, source)
	if err != nil {
		out.State, out.Details = "blocked", "Stored completion evidence is incomplete"
		return out
	}
	out.State, out.Details, out.Snapshot = "completed", r.Clearance.Reference, snapshot

	return out
}
