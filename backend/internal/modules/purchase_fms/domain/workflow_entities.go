package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type WorkflowTimestamp = time.Time
type WorkflowError = Error

func WorkflowInvalid(f, m string) error { return &WorkflowError{422, f, m} }

var WorkflowUnauthorized = &WorkflowError{401, "", "Invalid or expired credentials"}
var WorkflowForbidden = &WorkflowError{403, "", "Purchase package permission or task scope denied"}
var WorkflowMissing = &WorkflowError{404, "", "Purchase package or source not found"}
var WorkflowStale = &WorkflowError{409, "", "Package changed; retain your draft, reload and review before saving"}
var WorkflowDuplicate = &WorkflowError{409, "", "This released indent line already has a purchase package"}
var workflowUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func WorkflowID(v string) error {
	if !workflowUuid.MatchString(v) {
		return WorkflowInvalid("id", "Use a valid record identifier")
	}
	return nil
}
func WorkflowText(f, v string, max int, required bool) (string, error) {
	v = strings.TrimSpace(v)
	if (required && v == "") || utf8.RuneCountInString(v) > max {
		return v, WorkflowInvalid(f, "Use a nonempty value within the field limit")
	}
	return v, nil
}
func WorkflowDate(s string) bool {
	v, e := time.Parse("2006-01-02", s)
	return e == nil && v.Year() > 0 && v.Format("2006-01-02") == s
}

type WorkflowAccess struct{ Read, Write, Execute, Resolve, Recovery bool }
type WorkflowMember struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Execute bool   `json:"can_execute"`
	Resolve bool   `json:"can_resolve"`
}
type WorkflowSource struct {
	ProjectName  string            `json:"project_name"`
	Site         string            `json:"site"`
	IndentNumber string            `json:"indent_number"`
	DemandID     string            `json:"demand_id"`
	Description  string            `json:"description"`
	Quantity     string            `json:"quantity"`
	UOM          string            `json:"uom"`
	RequiredDate string            `json:"required_date"`
	FullKit      string            `json:"full_kit"`
	ReleaseAt    WorkflowTimestamp `json:"release_at"`
	Available    bool              `json:"project_available"`
	Eligible     bool              `json:"eligible"`
	Blockers     []string          `json:"blockers"`
}

func (s WorkflowSource) Guard() error {
	if !s.Available {
		return WorkflowInvalid("project", "Project is inactive, held, closed or purchasing is disabled")
	}
	if !s.Eligible || len(s.Blockers) > 0 {
		return WorkflowInvalid("source", "Purchase preparation blocked: "+strings.Join(s.Blockers, "; "))
	}
	return nil
}

type WorkflowCandidate struct {
	IndentID     string `json:"indent_id"`
	IndentNumber string `json:"indent_number"`
	DemandID     string `json:"demand_id"`
	Description  string `json:"description"`
	Quantity     string `json:"quantity"`
}
type WorkflowStep struct {
	Number            int                `json:"number"`
	Title             string             `json:"title"`
	Phase             string             `json:"phase"`
	Evidence          string             `json:"evidence_required"`
	Team              string             `json:"team"`
	DoerID            *string            `json:"doer_id"`
	ResolverID        *string            `json:"resolver_id"`
	PlannedDate       *string            `json:"planned_date"`
	State             string             `json:"state"`
	Notes             string             `json:"notes"`
	BlockReason       string             `json:"block_reason"`
	BlockedAt         *WorkflowTimestamp `json:"blocked_at"`
	BlockedResolverID *string            `json:"blocked_resolver_id"`
	ResolutionDue     *string            `json:"resolution_due"`
	UpdatedAt         WorkflowTimestamp  `json:"updated_at"`
	UpdatedBy         string             `json:"updated_by_id"`
}
type WorkflowPackage struct {
	ID          string            `json:"id"`
	ProjectID   string            `json:"project_id"`
	IndentID    string            `json:"indent_id"`
	DemandID    string            `json:"demand_id"`
	Code        string            `json:"code"`
	Description string            `json:"description"`
	Source      WorkflowSource    `json:"source_snapshot"`
	Version     int32             `json:"version"`
	Active      bool              `json:"is_active"`
	CreatedAt   WorkflowTimestamp `json:"created_at"`
	UpdatedAt   WorkflowTimestamp `json:"updated_at"`
	CreatedBy   *string           `json:"created_by_id"`
	UpdatedBy   *string           `json:"updated_by_id"`
	Steps       []WorkflowStep    `json:"steps"`
}

func WorkflowCopy(v WorkflowPackage) WorkflowPackage {
	v.Steps = append([]WorkflowStep{}, v.Steps...)
	v.Source.Blockers = append([]string{}, v.Source.Blockers...)
	return v
}

type WorkflowEvent struct {
	ID      string            `json:"id"`
	Action  string            `json:"action"`
	Reason  string            `json:"reason"`
	Before  *WorkflowPackage  `json:"before"`
	After   WorkflowPackage   `json:"after"`
	At      WorkflowTimestamp `json:"created_at"`
	ActorID string            `json:"created_by_id"`
}
type WorkflowListing struct {
	Total      int64               `json:"total"`
	Page       int32               `json:"page"`
	Size       int32               `json:"page_size"`
	Packages   []WorkflowSummary   `json:"packages"`
	Candidates []WorkflowCandidate `json:"candidates"`
	Members    []WorkflowMember    `json:"members"`
	CanCreate  bool                `json:"can_create"`
	ActorID    string              `json:"actor_id"`
}
type WorkflowWorkspace struct {
	Package   WorkflowPackage  `json:"package"`
	Source    WorkflowSource   `json:"source"`
	Members   []WorkflowMember `json:"members"`
	Actions   map[int][]string `json:"actions"`
	CanAssign bool             `json:"can_assign"`
	TaskOnly  bool             `json:"task_only"`
	History   []WorkflowEvent  `json:"history"`
}
type WorkflowAssignment struct {
	Number      int    `json:"number"`
	Team        string `json:"team"`
	DoerID      string `json:"doer_id"`
	ResolverID  string `json:"resolver_id"`
	PlannedDate string `json:"planned_date"`
}
type WorkflowCreate struct {
	IndentID string `json:"indent_id"`
	DemandID string `json:"demand_id"`
	Reason   string `json:"reason"`
}
type WorkflowAssign struct {
	Version int32                `json:"version"`
	Rows    []WorkflowAssignment `json:"rows"`
	Reason  string               `json:"reason"`
}
type WorkflowCommand struct {
	Version       int32   `json:"version"`
	Action        string  `json:"action"`
	Notes         string  `json:"notes"`
	Reason        string  `json:"reason"`
	ResolutionDue *string `json:"resolution_due"`
}
