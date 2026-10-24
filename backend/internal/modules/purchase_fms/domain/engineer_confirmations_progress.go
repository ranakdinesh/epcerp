package domain

// Progress is recorded work, never a downstream execution authorization.
type EngineerConfirmationsProgress struct {
	ID                string
	Version, Revision int32
	State, Details    string
	Current           bool
	Snapshot          *EngineerConfirmationsRecord `json:"-"`
}

func EngineerConfirmationsProgressFact(r *EngineerConfirmationsRecord, expected EngineerConfirmationsSourceFacts, upstreamCurrent bool) EngineerConfirmationsProgress {
	out := EngineerConfirmationsProgress{State: "not_started"}
	if r == nil || !r.Active {
		return out
	}
	out.ID, out.Version, out.Revision = r.ID, r.Version, r.Revision
	source := EngineerConfirmationsSource{EngineerConfirmationsSourceFacts: expected, Available: true}
	if !upstreamCurrent || source.Guard() != nil || r.ProjectID != expected.ProjectID || r.PackageID != expected.PackageID || r.Source.EngineerConfirmationsSourceFacts != expected {
		out.State, out.Details = "blocked", "Recorded evidence does not match current upstream records"
		return out
	}
	out.Current = true
	if r.Confirmation.Status == "pending" {
		return out
	}
	snapshot, err := EngineerConfirmationsConfirmedFact(r, source)
	if err != nil {
		out.State, out.Details = "blocked", "Stored completion evidence is incomplete"
		return out
	}
	out.State, out.Details, out.Snapshot = "completed", r.Confirmation.EngineerName, snapshot

	return out
}
