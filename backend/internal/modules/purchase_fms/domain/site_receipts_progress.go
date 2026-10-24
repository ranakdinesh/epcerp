package domain

// Progress is recorded work, never a downstream execution authorization.
type SiteReceiptsProgress struct {
	ID                string
	Version, Revision int32
	State, Details    string
	Current           bool
	Snapshot          *SiteReceiptsRecord `json:"-"`
}

func SiteReceiptsProgressFact(r *SiteReceiptsRecord, expected SiteReceiptsSourceFacts, upstreamCurrent bool) SiteReceiptsProgress {
	out := SiteReceiptsProgress{State: "not_started"}
	if r == nil || !r.Active {
		return out
	}
	out.ID, out.Version, out.Revision = r.ID, r.Version, r.Revision
	source := SiteReceiptsSource{SiteReceiptsSourceFacts: expected, Available: true}
	if !upstreamCurrent || source.Guard() != nil || r.ProjectID != expected.ProjectID || r.PackageID != expected.PackageID || r.Source.SiteReceiptsSourceFacts != expected {
		out.State, out.Details = "blocked", "Recorded evidence does not match current upstream records"
		return out
	}
	out.Current = true
	if r.Receipt.Status == "pending" || r.Receipt.Status == "in_transit" {
		return out
	}
	snapshot, err := SiteReceiptsDeliveredFact(r, source)
	if err != nil {
		out.State, out.Details = "blocked", "Stored completion evidence is incomplete"
		return out
	}
	out.State, out.Details, out.Snapshot = "completed", r.Receipt.ArrivalDate, snapshot

	return out
}
