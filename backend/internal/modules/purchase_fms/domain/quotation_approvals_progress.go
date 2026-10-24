package domain

// Progress exposes statuses only; no budget values, reviewer comments or authority.
type QuotationApprovalsProgressGroup struct {
	ID             string
	ScopeRevision  int32
	State, Details string
}
type QuotationApprovalsProgress struct {
	ProjectID, BOID  string
	Started, Ready   bool
	ApprovedGroupIDs []string
	Groups           []QuotationApprovalsProgressGroup
}

func QuotationApprovalsProgressFacts(r *QuotationApprovalsRecord, s QuotationApprovalsSource) (QuotationApprovalsProgress, error) {
	out := QuotationApprovalsProgress{ProjectID: s.ProjectID, BOID: s.BOID, ApprovedGroupIDs: []string{}, Groups: []QuotationApprovalsProgressGroup{}}
	if r != nil && r.Active && (r.ProjectID != s.ProjectID || r.BOID != s.BOID) {
		return out, QuotationApprovalsInvalid("source", "Approval report record must match the current project and BO")
	}
	// A read preserves recorded approval through a project hold. Effective quote,
	// group revision and vendor proof checks remain unchanged. Budget is advisory.
	s.Available = true
	latest := map[string]QuotationApprovalsAttempt{}
	if r != nil && r.Active {
		_, latest, _ = QuotationApprovalsEvaluate(r.Attempts, s)
	}
	seen := map[string]bool{}
	for _, g := range s.Groups {
		if g.ID == "" || seen[g.ID] || g.ScopeRevision < 1 {
			return QuotationApprovalsProgress{}, QuotationApprovalsInvalid("groups", "Use distinct current group revisions")
		}
		seen[g.ID] = true
		row := QuotationApprovalsProgressGroup{ID: g.ID, ScopeRevision: g.ScopeRevision, State: "not_started"}
		if a, exists := latest[g.ID]; exists {
			out.Started = true
			row.State, row.Details = a.EffectiveState, a.Message
			valid := a.ScopeRevision == g.ScopeRevision && a.DecisionID != nil && a.ReviewedBy != nil && a.ReviewedAt != nil && !a.ReviewedAt.IsZero() && *a.ReviewedBy != a.SubmittedBy && len(a.Lines) > 0
			if valid {
				_, e := QuotationApprovalsID(*a.DecisionID)
				valid = e == nil
				_, e = QuotationApprovalsID(*a.ReviewedBy)
				valid = valid && e == nil
			}
			for _, line := range a.Lines {
				if line.Decision == nil || *line.Decision != "accept" {
					valid = false
				}
			}
			if row.State == "approved" && !valid {
				row.State, row.Details = "stale", "Actual independent item approval evidence is incomplete"
			}
			if row.State == "approved" {
				out.ApprovedGroupIDs = append(out.ApprovedGroupIDs, g.ID)
			}
		}
		out.Groups = append(out.Groups, row)
	}
	out.Ready = len(out.Groups) > 0 && len(out.ApprovedGroupIDs) == len(out.Groups)
	return out, nil
}
