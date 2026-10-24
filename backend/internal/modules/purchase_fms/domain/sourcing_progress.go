package domain

// Progress represents recorded sourcing work, never current commercial authority.
type SourcingProgressStep struct {
	Number         int
	State, Details string
}
type SourcingProgress struct {
	CaseID   string
	Version  int32
	Current  bool
	Steps    []SourcingProgressStep
	Snapshot *SourcingReleased `json:"-"`
}

func SourcingProgressFacts(c *SourcingCase, project, pkg string, expected SourcingSource, upstreamCurrent bool) SourcingProgress {
	out := SourcingProgress{Steps: []SourcingProgressStep{{Number: 3, State: "not_started"}, {Number: 4, State: "not_started"}, {Number: 5, State: "not_started"}}}
	if c == nil || !c.Active {
		return out
	}
	out.CaseID, out.Version = c.ID, c.Version
	expected.Available, expected.Eligible, expected.Blockers = true, true, nil
	block := func(details string) SourcingProgress {
		for i := range out.Steps {
			out.Steps[i].State, out.Steps[i].Details = "blocked", details
		}
		return out
	}
	if !upstreamCurrent || expected.Guard() != nil || expected.PackageID != pkg || c.ProjectID != project || c.PackageID != pkg || !sourcingSameSource(c.Source, expected) {
		return block("Sourcing evidence does not match the current item demand")
	}
	if c.Decision != nil && !sourcingSameSource(c.Decision.Source, expected) {
		return block("Selection evidence belongs to an obsolete demand revision")
	}
	out.Current = true
	for i := range out.Steps {
		out.Steps[i].State = "in_progress"
	}
	active, sent := 0, 0
	for _, offer := range c.Offers {
		if !offer.Active {
			continue
		}
		active++
		_, dateErr := SourcingDate("sent_date", offer.SentDate)
		if dateErr == nil && offer.Channel != "" && offer.DispatchReference != "" {
			sent++
		}
	}
	if active > 0 && sent == active {
		out.Steps[0].State = "completed"
	}
	switch c.State {
	case "draft":
	case "pending", "approved", "released":
		if c.Decision == nil {
			out.Current = false
			return block("Recorded selection has no decision snapshot")
		}
		out.Steps[1].State = "completed"
	default:
		out.Current = false
		return block("Stored sourcing state is invalid")
	}
	if c.State == "released" {
		fact, err := SourcingReleasedFact(*c)
		if err != nil || fact == nil {
			out.Steps[2].State, out.Steps[2].Details = "blocked", "Stored LOI release evidence is incomplete"
			return out
		}
		_, dateErr := SourcingDate("loi_sent_date", c.LOISentDate)
		if dateErr != nil || c.LOIChannel == "" || c.LOIDispatchReference == "" {
			out.Steps[2].State, out.Steps[2].Details = "blocked", "Actual LOI dispatch evidence is missing"
			return out
		}
		out.Steps[2].State, out.Steps[2].Details, out.Snapshot = "completed", fact.LOIReference, fact
	}
	return out
}
