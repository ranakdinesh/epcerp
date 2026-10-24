package domain

import (
	"testing"
)

func TestWorkspaceOperationalScopeDoesNotPromotePortfolioReaders(t *testing.T) {
	project := WorkspaceProject{ID: "project", ManagerID: "pm", Active: true, Lifecycle: "draft"}
	workspace := Workspace{ProjectID: project.ID, ManagerID: "purchaser", Active: true}
	for _, tc := range []struct {
		principal Principal
		allowed   bool
	}{
		{Principal{ID: "pm", Active: true}, true},
		{Principal{ID: "purchaser", Active: true}, true},
		{Principal{ID: "unrelated", Active: true}, false},
		{Principal{ID: "unrelated", Active: true, Permissions: []string{"projects.purchase_portfolio.read"}}, false},
		{Principal{ID: "unrelated", Active: true, Permissions: []string{"projects.purchase_portfolio.write"}}, true},
		{Principal{ID: "unrelated", Active: true, Superuser: true}, true},
		{Principal{ID: "purchaser", Active: false}, false},
	} {
		if got := WorkspaceCanExecute(tc.principal, project, workspace); got != tc.allowed {
			t.Fatalf("%s: %v", tc.principal.ID, got)
		}
	}
	workspace.Active = false
	if WorkspaceCanExecute(Principal{ID: "purchaser", Active: true}, project, workspace) {
		t.Fatal("inactive purchase ownership allowed execution")
	}
}
