package domain

// Access is a scoped consumer view, never a grant of global permissions.
type WorkspaceAccess struct {
	ProjectID, ClientID, ProjectManagerID, ProcurementExecutiveID, ProjectName, ProjectNumber string
	CanWrite                                                                                  bool
}

func WorkspaceCanExecute(p Principal, project WorkspaceProject, w Workspace) bool {
	return p.Active && (p.Can("projects.purchase_portfolio.write") || project.ManagerID == p.ID || (w.Active && w.ManagerID == p.ID))
}

func WorkspaceCanManageHold(p Principal, project WorkspaceProject, w Workspace) bool {
	return WorkspaceCanExecute(p, project, w) && project.Active && project.Lifecycle != "closed"
}
