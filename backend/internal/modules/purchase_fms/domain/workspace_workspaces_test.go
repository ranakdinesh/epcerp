package domain

import (
	"testing"
)

func TestWorkspaceScopedOwnerAndPortfolioPermissions(t *testing.T) {
	p := Principal{ID: "buyer", Active: true, Permissions: []string{"projects.purchase_workflow.read"}}
	project := WorkspaceProject{ManagerID: "pm", Active: true}
	w := Workspace{ManagerID: "buyer", Active: true}
	if !WorkspaceCanRead(p, project, w) {
		t.Fatal("assigned owner denied")
	}
	w.ManagerID = "other"
	if WorkspaceCanRead(p, project, w) {
		t.Fatal("global permission granted unrelated project")
	}
	p.ID = "pm"
	if !WorkspaceCanRead(p, project, w) {
		t.Fatal("PM oversight denied")
	}
	if WorkspaceCanAssign(p, project) {
		t.Fatal("read granted assignments")
	}
	p.Permissions = append(p.Permissions, "projects.purchase_assignments.write")
	if !WorkspaceCanAssign(p, project) {
		t.Fatal("scoped manager denied")
	}
	p.ID = "buyer"
	if WorkspaceCanAssign(p, project) {
		t.Fatal("purchase owner became project administrator")
	}
	p.Active = false
	p.Superuser = true
	if WorkspaceCanRead(p, project, w) || WorkspaceCanAssign(p, project) {
		t.Fatal("inactive superuser allowed")
	}
}
func TestWorkspaceCandidateAndInputValidation(t *testing.T) {
	c := WorkspaceCandidate{ID: "candidate", Active: true, Permissions: []string{"projects.purchase_workflow.view"}}
	if c.Eligible() {
		t.Fatal("view alone eligible")
	}
	c.Permissions = append(c.Permissions, "projects.purchase_workflow.read")
	if !c.Eligible() {
		t.Fatal("eligible reader rejected")
	}
	c.Active = false
	if c.Eligible() {
		t.Fatal("inactive candidate")
	}
	change := WorkspaceChange{ManagerID: "11111111-1111-4111-8111-111111111111", Reason: "first owner", Version: 0}
	if _, e := change.Validate(); e != nil {
		t.Fatal(e)
	}
	change.Version = -1
	if _, e := change.Validate(); e == nil {
		t.Fatal("negative revision")
	}
	if WorkspaceAssignmentAllowed(WorkspaceProject{Active: true, Lifecycle: "on_hold"}) {
		t.Fatal("hold owner changes allowed")
	}
	if _, e := (WorkspaceFilter{Page: 1, Size: 20, Sort: "arbitrary_sql"}).Validate(); e == nil {
		t.Fatal("unknown sort")
	}
}
