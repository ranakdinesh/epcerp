package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var WorkspaceUnauthorized = errors.New("invalid or expired credentials")
var WorkspaceForbidden = errors.New("purchase project access denied")
var WorkspaceMissing = errors.New("purchase project not found")
var WorkspaceIneligible = errors.New("Select an active Procurement Executive with purchase view/read permissions")
var WorkspaceStale = errors.New("purchase ownership changed; refresh before saving")

type WorkspaceInvalid struct{ Field, Message string }

func (e *WorkspaceInvalid) Error() string { return e.Message }

var workspaceUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func WorkspaceID(id string) (string, error) {
	id = strings.ToLower(id)
	if !workspaceUuid.MatchString(id) {
		return "", &WorkspaceInvalid{"id", "Select a valid record"}
	}
	return id, nil
}

func WorkspacePortfolio(p Principal) bool {
	return p.Can("projects.purchase_portfolio.read") || p.Can("projects.purchase_portfolio.write")
}

type WorkspaceProject struct {
	ID, Name, Number, ManagerID, Lifecycle string
	Active                                 bool
}
type Workspace struct {
	ProjectID, ManagerID string
	Version              int32
	Active               bool
	CreatedAt, UpdatedAt time.Time
	CreatedBy, UpdatedBy string
}
type WorkspaceCandidate struct {
	ID, Name          string
	Active, Superuser bool
	Permissions       []string
}

func (c WorkspaceCandidate) Eligible() bool {
	p := Principal{ID: c.ID, Active: c.Active, Superuser: c.Superuser, Permissions: c.Permissions}
	return p.Can("projects.purchase_workflow.view") && p.Can("projects.purchase_workflow.read")
}
func WorkspaceCanRead(p Principal, project WorkspaceProject, w Workspace) bool {
	return p.Active && (WorkspacePortfolio(p) || project.ManagerID == p.ID || (w.Active && w.ManagerID == p.ID))
}
func WorkspaceCanAssign(p Principal, project WorkspaceProject) bool {
	return p.Can("projects.purchase_assignments.write") && (p.Can("projects.purchase_portfolio.write") || project.ManagerID == p.ID)
}
func WorkspaceAssignmentAllowed(project WorkspaceProject) bool {
	return project.Active && project.Lifecycle != "closed" && project.Lifecycle != "on_hold"
}

type WorkspaceChange struct {
	ManagerID string `json:"purchase_manager_id"`
	Version   int32  `json:"expected_version"`
	Reason    string `json:"reason"`
}

func (c WorkspaceChange) Validate() (WorkspaceChange, error) {
	var e error
	if c.ManagerID, e = WorkspaceID(c.ManagerID); e != nil {
		return c, &WorkspaceInvalid{"purchase_manager_id", "Select an eligible purchase manager"}
	}
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Reason == "" || utf8.RuneCountInString(c.Reason) > 2000 {
		return c, &WorkspaceInvalid{"reason", "Enter a reason within 2000 characters"}
	}
	if c.Version < 0 {
		return c, &WorkspaceInvalid{"expected_version", "Use the current ownership version"}
	}
	return c, nil
}

type WorkspaceFilter struct {
	Page, Size              int32
	Search, Sort, Direction string
}

func (f WorkspaceFilter) Validate() (WorkspaceFilter, error) {
	if f.Page < 1 || f.Page > 100000 || f.Size < 1 || f.Size > 100 || utf8.RuneCountInString(f.Search) > 255 {
		return f, &WorkspaceInvalid{"query", "Invalid pagination or search"}
	}
	if f.Sort == "" {
		f.Sort = "number"
	}
	if f.Direction == "" {
		f.Direction = "asc"
	}
	if (f.Sort != "number" && f.Sort != "name" && f.Sort != "state") || (f.Direction != "asc" && f.Direction != "desc") {
		return f, &WorkspaceInvalid{"query", "Unsupported sort"}
	}
	f.Search = strings.ToLower(strings.TrimSpace(f.Search))
	return f, nil
}

type WorkspaceHistory struct {
	ID, ProjectID, PreviousID, ManagerID, ActorID, Reason string
	Version                                               int32
	CreatedAt                                             time.Time
}
type WorkspaceRow struct {
	Project   WorkspaceProject
	Workspace Workspace
	CanAssign bool
	Progress  RegisterProgress
}
type WorkspacePage struct {
	Rows       []WorkspaceRow
	Total      int64
	Page, Size int32
	Portfolio  bool
}
type WorkspaceContext struct {
	Row     WorkspaceRow
	History []WorkspaceHistory
}
