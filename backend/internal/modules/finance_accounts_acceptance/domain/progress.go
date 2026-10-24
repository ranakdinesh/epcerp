package domain

// Progress is recorded work, never a downstream execution authorization.
type Progress struct {
	ID                string
	Version, Revision int32
	State, Details    string
	Current           bool
	Snapshot          *Record `json:"-"`
}

func ProgressFact(r *Record, expected SourceFacts, upstreamCurrent bool) Progress {
	out := Progress{State: "not_started"}
	if r == nil || !r.Active {
		return out
	}
	out.ID, out.Version, out.Revision = r.ID, r.Version, r.Revision
	source := Source{SourceFacts: expected, Available: true}
	if !upstreamCurrent || source.Guard() != nil || r.ProjectID != expected.ProjectID || r.PackageID != expected.PackageID || r.Source.SourceFacts != expected {
		out.State, out.Details = "blocked", "Recorded evidence does not match current upstream records"
		return out
	}
	out.Current = true
	if r.Acceptance.Status == "pending" {
		return out
	}
	snapshot, err := AcceptedFact(r, source)
	if err != nil {
		out.State, out.Details = "blocked", "Stored completion evidence is incomplete"
		return out
	}
	out.State, out.Details, out.Snapshot = "completed", r.Acceptance.OfficerName, snapshot

	return out
}
