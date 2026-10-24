package domain

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type Timestamp = time.Time
type Error struct {
	Status         int
	Field, Message string
}

func (e *Error) Error() string { return e.Message }

var Unauthorized = &Error{401, "", "Authentication required"}
var Forbidden = &Error{403, "", "Current personal task permissions required"}

func Invalid(f, m string) error { return &Error{422, f, m} }

type Actor struct {
	ID                string
	Active, Superuser bool
	Permissions       []string
}

func (a Actor) Can(code string) bool {
	return a.Active && a.ID != "" && (a.Superuser || slices.Contains(a.Permissions, code))
}

type Query struct {
	Page, Size          int32
	Search, Role, State string
}

func (q Query) Validate() (Query, error) {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Size == 0 {
		q.Size = 20
	}
	if q.Role == "" {
		q.Role = "all"
	}
	if q.State == "" {
		q.State = "all"
	}
	q.Search = strings.TrimSpace(q.Search)
	if q.Page < 1 || q.Page > 100000 || !slices.Contains([]int32{20, 50, 100}, q.Size) || !utf8.ValidString(q.Search) || utf8.RuneCountInString(q.Search) > 120 {
		return q, Invalid("query", "Use bounded page/search and 20/50/100 rows")
	}
	if !slices.Contains([]string{"all", "doer", "resolver"}, q.Role) || !slices.Contains([]string{"all", "not_started", "preparing", "blocked"}, q.State) {
		return q, Invalid("query", "Choose a supported role and coordination state")
	}
	return q, nil
}

type Task struct {
	ID            string   `json:"id"`
	ProjectID     string   `json:"project_id"`
	ProjectName   string   `json:"project_name"`
	PackageID     string   `json:"package_id"`
	Code          string   `json:"package_code"`
	Number        int      `json:"step_number"`
	Title         string   `json:"title"`
	Phase         string   `json:"phase"`
	Relation      string   `json:"relation"`
	State         string   `json:"coordination_state"`
	PlannedDate   string   `json:"planned_date"`
	ResolutionDue string   `json:"resolution_due"`
	BlockReason   string   `json:"block_reason"`
	Available     bool     `json:"available"`
	Blockers      []string `json:"blockers"`
	Version       int32    `json:"package_version"`
	Overdue       bool     `json:"is_overdue"`
}

func (t Task) Due() string {
	if t.Relation == "resolver" {
		return t.ResolutionDue
	}
	return t.PlannedDate
}
func (t Task) WithClock(now Timestamp) Task {
	today := now.In(time.FixedZone("India", 19800)).Format("2006-01-02")
	t.Overdue = t.Due() != "" && t.Due() < today
	t.Blockers = append([]string{}, t.Blockers...)
	return t
}

type Page struct {
	SupplierReviews []SupplierReviewTask `json:"supplier_review_tasks"`
	Quotations      []QuotationTask      `json:"quotation_tasks"`
	Rows            []Task               `json:"rows"`
	Total           int64                `json:"total"`
	Page            int32                `json:"page"`
	Size            int32                `json:"page_size"`
}

type QuotationTask struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	Title       string `json:"title"`
	Link        string `json:"link"`
	State       string `json:"state"`
}

type SupplierReviewTask struct {
	ID         string `json:"id"`
	DocumentID string `json:"document_id"`
	Title      string `json:"title"`
	Kind       string `json:"kind"`
	Outcome    string `json:"outcome"`
	Completed  bool   `json:"completed"`
	SupplierID string `json:"supplier_id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	State      string `json:"state"`
	Message    string `json:"message"`
	Link       string `json:"link"`
}
