package domain

import (
	"testing"
	"time"
)

const workflowWorker = "11111111-1111-4111-8111-111111111111"
const workflowResolver = "22222222-2222-4222-8222-222222222222"

func workflowAssigned(t *testing.T) WorkflowPackage {
	t.Helper()
	p, e := WorkflowApplyAssignments(WorkflowPackage{Steps: WorkflowDefinitions()}, []WorkflowAssignment{{Number: 5, Team: "Procurement", DoerID: workflowWorker, ResolverID: workflowResolver, PlannedDate: "2026-10-10"}}, []WorkflowMember{{ID: workflowWorker, Execute: true}, {ID: workflowResolver, Resolve: true}}, workflowWorker, time.Time{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestWorkflowCapturedBlockerAndIndianDeadline(t *testing.T) {
	p := workflowAssigned(t)
	now := time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)
	old := "2026-10-09"
	a := Principal{ID: workflowWorker, Active: true}
	caps := WorkflowAccess{Execute: true, Resolve: true}
	if _, e := WorkflowApplyCommand(p, 5, WorkflowCommand{Action: "block", Reason: "Missing drawing", ResolutionDue: &old}, a, caps, now); e == nil {
		t.Fatal("Indian yesterday accepted")
	}
	due := "2026-10-10"
	blocked, e := WorkflowApplyCommand(p, 5, WorkflowCommand{Action: "block", Reason: "Missing drawing", ResolutionDue: &due}, a, caps, now)
	if e != nil || blocked.Steps[4].BlockedResolverID == nil || *blocked.Steps[4].BlockedResolverID != workflowResolver || p.Steps[4].State != "not_started" {
		t.Fatal("blocker capture/copy", e)
	}
	if _, e = WorkflowApplyAssignments(blocked, []WorkflowAssignment{{Number: 5, Team: "Changed", DoerID: workflowResolver, ResolverID: workflowWorker, PlannedDate: due}}, []WorkflowMember{{ID: workflowWorker, Resolve: true}, {ID: workflowResolver, Execute: true}}, workflowWorker, now); e == nil {
		t.Fatal("blocked reassignment")
	}
	if _, e = WorkflowApplyCommand(blocked, 5, WorkflowCommand{Action: "resolve"}, a, caps, now); e == nil {
		t.Fatal("doer resolved captured blocker")
	}
	resolved, e := WorkflowApplyCommand(blocked, 5, WorkflowCommand{Action: "resolve"}, Principal{ID: workflowResolver, Active: true}, caps, now)
	if e != nil || resolved.Steps[4].State != "preparing" || resolved.Steps[4].BlockedAt != nil || resolved.Steps[4].ResolutionDue != nil {
		t.Fatal(e)
	}
}
func TestWorkflowNoPreparationCompletionOrStaleRecoveryBypass(t *testing.T) {
	p := workflowAssigned(t)
	for _, action := range []string{"complete", "approve", "release"} {
		if _, e := WorkflowApplyCommand(p, 5, WorkflowCommand{Action: action}, Principal{ID: workflowWorker, Active: true}, WorkflowAccess{Execute: true}, time.Time{}); e == nil {
			t.Fatal("commercial completion inferred")
		}
	}
	if e := WorkflowWorkGuard(Principal{ID: workflowResolver, Active: true, Superuser: true}, WorkflowAccess{Execute: true}, p.Steps[4], "start"); e == nil {
		t.Fatal("unverified recovery claim bypassed assignment")
	}
	if e := WorkflowWorkGuard(Principal{ID: workflowWorker, Active: true}, WorkflowAccess{Execute: true}, p.Steps[0], "start"); e == nil {
		t.Fatal("source step editable")
	}
	if e := (WorkflowSource{Available: true, Eligible: true, Blockers: []string{"Hold"}}).Guard(); e == nil {
		t.Fatal("hold ignored")
	}
}
func TestWorkflowAssignmentValidation(t *testing.T) {
	for _, q := range []WorkflowAssignment{{Number: 2, Team: "P", DoerID: workflowWorker, ResolverID: workflowResolver, PlannedDate: "2026-10-10"}, {Number: 5, Team: "P", DoerID: workflowWorker, ResolverID: workflowWorker, PlannedDate: "2026-10-10"}, {Number: 5, Team: "P", DoerID: workflowWorker, ResolverID: workflowResolver, PlannedDate: "bad"}} {
		if _, e := WorkflowApplyAssignments(WorkflowPackage{Steps: WorkflowDefinitions()}, []WorkflowAssignment{q}, []WorkflowMember{{ID: workflowWorker, Execute: true}, {ID: workflowResolver, Resolve: true}}, workflowWorker, time.Time{}); e == nil {
			t.Fatal("invalid assignment accepted", q)
		}
	}
}
