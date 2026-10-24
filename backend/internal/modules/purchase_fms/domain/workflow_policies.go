package domain

import (
	"time"
)

func WorkflowAssigned(s WorkflowStep, a Principal, action string) bool {
	p := s.DoerID
	if action == "resolve" {
		p = s.BlockedResolverID
	}
	return p != nil && *p == a.ID
}
func WorkflowWorkGuard(a Principal, caps WorkflowAccess, s WorkflowStep, action string) error {
	if s.Number < 3 || s.Number > 20 {
		return WorkflowInvalid("step", "Steps1–2 are linked source records; use item and indent controls")
	}
	if action != "save" && action != "start" && action != "block" && action != "resolve" {
		return WorkflowInvalid("action", "Specialist completion is not inferred from preparation")
	}
	allowed := caps.Execute
	if action == "resolve" {
		allowed = caps.Resolve
	}
	if !allowed || (!WorkflowAssigned(s, a, action) && !caps.Recovery) {
		return WorkflowForbidden
	}
	if s.DoerID == nil || s.ResolverID == nil || *s.DoerID == *s.ResolverID || s.Team == "" || s.PlannedDate == nil {
		return WorkflowInvalid("assignment", "Assign team, distinct doer/resolver and planned date first")
	}
	if action == "resolve" && s.State != "blocked" {
		return WorkflowInvalid("step", "This step has no open blocker")
	}
	if action != "resolve" && s.State == "blocked" {
		return WorkflowInvalid("step", "Resolve the captured blocker before editing preparation")
	}
	return nil
}
func WorkflowApplyAssignments(p WorkflowPackage, rows []WorkflowAssignment, members []WorkflowMember, actor string, now WorkflowTimestamp) (WorkflowPackage, error) {
	if len(p.Steps) != 20 {
		return p, WorkflowInvalid("package", "Invalid package step catalogue")
	}
	if len(rows) < 1 || len(rows) > 18 {
		return p, WorkflowInvalid("rows", "Assign1–18 preparation steps")
	}
	seen := map[int]bool{}
	p = WorkflowCopy(p)
	for i, r := range rows {
		if r.Number < 3 || r.Number > 20 || seen[r.Number] {
			return p, WorkflowInvalid("rows", "Use unique preparation steps3–20")
		}
		seen[r.Number] = true
		if WorkflowID(r.DoerID) != nil || WorkflowID(r.ResolverID) != nil || r.DoerID == r.ResolverID {
			return p, WorkflowInvalid("resolver_id", "Choose distinct active doer and resolver")
		}
		team, e := WorkflowText("team", r.Team, 120, true)
		if e != nil {
			return p, e
		}
		if !WorkflowDate(r.PlannedDate) {
			return p, WorkflowInvalid("planned_date", "Choose a planned date")
		}
		doer, resolver := false, false
		for _, m := range members {
			doer = doer || (m.ID == r.DoerID && m.Execute)
			resolver = resolver || (m.ID == r.ResolverID && m.Resolve)
		}
		if !doer || !resolver {
			return p, WorkflowInvalid("rows", "Choose active users with live matching workflow permissions")
		}
		s := &p.Steps[r.Number-1]
		if s.State == "blocked" {
			return p, WorkflowInvalid("rows", "Resolve the captured blocker before reassignment")
		}
		rows[i].Team = team
		s.Team = team
		s.DoerID = &rows[i].DoerID
		s.ResolverID = &rows[i].ResolverID
		s.PlannedDate = &rows[i].PlannedDate
		s.UpdatedAt = now
		s.UpdatedBy = actor
	}
	return p, nil
}
func WorkflowApplyCommand(p WorkflowPackage, number int, c WorkflowCommand, a Principal, caps WorkflowAccess, now WorkflowTimestamp) (WorkflowPackage, error) {
	if len(p.Steps) != 20 {
		return p, WorkflowInvalid("package", "Invalid package step catalogue")
	}
	if number < 1 || number > 20 {
		return p, WorkflowInvalid("step", "Unknown purchase step")
	}
	p = WorkflowCopy(p)
	s := &p.Steps[number-1]
	if e := WorkflowWorkGuard(a, caps, *s, c.Action); e != nil {
		return p, e
	}
	notes, e := WorkflowText("notes", c.Notes, 4000, false)
	if e != nil {
		return p, e
	}
	if c.Action == "block" {
		if c.ResolutionDue == nil || !WorkflowDate(*c.ResolutionDue) || *c.ResolutionDue < now.In(workflowTimeLocation()).Format("2006-01-02") {
			return p, WorkflowInvalid("resolution_due", "A resolver deadline today or later is required")
		}
		s.State = "blocked"
		s.BlockReason = c.Reason
		s.Notes = notes
		s.BlockedAt = &now
		captured := *s.ResolverID
		s.BlockedResolverID = &captured
		due := *c.ResolutionDue
		s.ResolutionDue = &due
	} else if c.Action == "resolve" {
		s.State = "preparing"
		s.BlockReason = ""
		s.BlockedAt = nil
		s.BlockedResolverID = nil
		s.ResolutionDue = nil
	} else {
		s.State = "preparing"
		s.Notes = notes
	}
	s.UpdatedAt = now
	s.UpdatedBy = a.ID
	return p, nil
}

func workflowTimeLocation() *time.Location { return time.FixedZone("Asia/Kolkata", 19800) }
