package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type TechnicalTimestamp = time.Time
type TechnicalError = Error

func TechnicalInvalid(f, m string) error { return &TechnicalError{422, f, m} }

var TechnicalUnauthorized = &TechnicalError{401, "", "Invalid or expired credentials"}
var TechnicalForbidden = &TechnicalError{403, "", "Technical permission or task scope denied"}
var TechnicalMissing = &TechnicalError{404, "", "Technical package or evidence not found"}
var TechnicalStale = &TechnicalError{409, "", "Technical package changed; retain the draft, reload and review"}
var technicalUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TechnicalID(s string) error {
	if !technicalUuid.MatchString(s) {
		return TechnicalInvalid("id", "Use a valid record identifier")
	}
	return nil
}
func TechnicalText(f, s string, n int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > n || (required && s == "") {
		return s, TechnicalInvalid(f, "Use a nonempty value within the field limit")
	}
	return s, nil
}
func TechnicalDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}
func TechnicalToday(t time.Time) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}

type TechnicalAccess struct {
	Read, Write, Approve, Scoped bool
	Work                         map[int]bool
}
type TechnicalSource struct {
	SupplierName       string   `json:"supplier_name,omitempty"`
	CaseID             string   `json:"case_id"`
	Version            int32    `json:"version"`
	ProjectID          string   `json:"project_id"`
	PackageID          string   `json:"package_id"`
	LOIReference       string   `json:"loi_reference"`
	SupplierID         string   `json:"supplier_id"`
	Description        string   `json:"description"`
	Quantity           string   `json:"quantity"`
	Currency           string   `json:"currency"`
	Total              string   `json:"total"`
	FinancialReference string   `json:"financial_reference"`
	ReleasedDate       string   `json:"released_date"`
	Available          bool     `json:"project_available"`
	Blockers           []string `json:"blockers"`
}
type TechnicalDocument struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	Standard     string `json:"standard"`
	Required     bool   `json:"required"`
	Status       string `json:"status"`
	Revision     string `json:"revision"`
	ReceivedDate string `json:"received_date"`
	FileID       string `json:"file_id"`
	Notes        string `json:"notes"`
	NAReason     string `json:"na_reason"`
	Active       bool   `json:"is_active"`
}
type TechnicalFile struct {
	ID        string `json:"id"`
	CaseID    string `json:"case_id"`
	Purpose   string `json:"purpose"`
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Active    bool   `json:"is_active"`
}
type TechnicalSubmission struct {
	Revision    int32               `json:"revision"`
	Reference   string              `json:"reference"`
	Date        string              `json:"date"`
	TargetDate  string              `json:"target_date"`
	Authority   string              `json:"authority"`
	FileID      string              `json:"file_id"`
	Documents   []TechnicalDocument `json:"documents"`
	SubmittedBy string              `json:"submitted_by_id"`
	SubmittedAt TechnicalTimestamp  `json:"submitted_at"`
}
type TechnicalTeamReview struct {
	Revision   int32              `json:"revision"`
	Approved   bool               `json:"approved"`
	Remarks    string             `json:"remarks"`
	ReviewerID string             `json:"reviewer_id"`
	ReviewedAt TechnicalTimestamp `json:"reviewed_at"`
}
type TechnicalApproval struct {
	Revision         int32              `json:"revision"`
	Status           string             `json:"status"`
	Reference        string             `json:"reference"`
	Date             string             `json:"date"`
	Name             string             `json:"approver_name"`
	Designation      string             `json:"approver_designation"`
	FileID           string             `json:"file_id"`
	QAPApproved      bool               `json:"qap_approved"`
	ConditionsClosed bool               `json:"conditions_closed"`
	ClosureFileID    string             `json:"closure_file_id"`
	Remarks          string             `json:"remarks"`
	RecordedBy       string             `json:"recorded_by_id"`
	RecordedAt       TechnicalTimestamp `json:"recorded_at"`
}
type TechnicalCase struct {
	ID         string               `json:"id"`
	ProjectID  string               `json:"project_id"`
	PackageID  string               `json:"package_id"`
	Source     TechnicalSource      `json:"source_snapshot"`
	Version    int32                `json:"version"`
	Revision   int32                `json:"revision"`
	State      string               `json:"state"`
	Documents  []TechnicalDocument  `json:"documents"`
	Submission *TechnicalSubmission `json:"submission"`
	TeamReview *TechnicalTeamReview `json:"team_review"`
	Approval   *TechnicalApproval   `json:"approval"`
	Active     bool                 `json:"is_active"`
	CreatedAt  TechnicalTimestamp   `json:"created_at"`
	UpdatedAt  TechnicalTimestamp   `json:"updated_at"`
	CreatedBy  string               `json:"created_by_id"`
	UpdatedBy  string               `json:"updated_by_id"`
}
type TechnicalCommand struct {
	Version          int32                     `json:"version"`
	Action           string                    `json:"action"`
	Reason           string                    `json:"reason"`
	Document         *TechnicalDocumentInput   `json:"document,omitempty"`
	Submission       *TechnicalSubmissionInput `json:"submission,omitempty"`
	Approve          *bool                     `json:"approve,omitempty"`
	Remarks          string                    `json:"remarks,omitempty"`
	Approval         *TechnicalApprovalInput   `json:"approval,omitempty"`
	QAPApproved      *bool                     `json:"qap_approved,omitempty"`
	ConditionsClosed *bool                     `json:"conditions_closed,omitempty"`
}
type TechnicalEvent struct {
	ID      string             `json:"id"`
	CaseID  string             `json:"case_id"`
	Action  string             `json:"action"`
	Reason  string             `json:"reason"`
	Before  *TechnicalCase     `json:"before"`
	After   TechnicalCase      `json:"after"`
	At      TechnicalTimestamp `json:"at"`
	ActorID string             `json:"actor_id"`
}
type TechnicalWorkspace struct {
	Case     *TechnicalCase   `json:"case"`
	Source   TechnicalSource  `json:"source"`
	Files    []TechnicalFile  `json:"files"`
	Actions  []string         `json:"actions"`
	History  []TechnicalEvent `json:"history"`
	Ready    bool             `json:"ready"`
	TaskOnly bool             `json:"task_only"`
}

func TechnicalCopy(c TechnicalCase) TechnicalCase {
	c.Source.Blockers = append([]string{}, c.Source.Blockers...)
	c.Documents = append([]TechnicalDocument{}, c.Documents...)
	if c.Submission != nil {
		v := *c.Submission
		v.Documents = append([]TechnicalDocument{}, v.Documents...)
		c.Submission = &v
	}
	if c.TeamReview != nil {
		v := *c.TeamReview
		c.TeamReview = &v
	}
	if c.Approval != nil {
		v := *c.Approval
		c.Approval = &v
	}
	return c
}

type TechnicalDocumentInput struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	Standard     string `json:"standard"`
	Required     *bool  `json:"required"`
	Status       string `json:"status"`
	Revision     string `json:"revision"`
	ReceivedDate string `json:"received_date"`
	FileID       string `json:"file_id"`
	Notes        string `json:"notes"`
	NAReason     string `json:"na_reason"`
	Active       *bool  `json:"is_active"`
}
type TechnicalSubmissionInput struct {
	Reference  string `json:"reference"`
	Date       string `json:"date"`
	TargetDate string `json:"target_date"`
	Authority  string `json:"authority"`
	FileID     string `json:"file_id"`
}
type TechnicalApprovalInput struct {
	Status        string `json:"status"`
	Reference     string `json:"reference"`
	Date          string `json:"date"`
	Name          string `json:"approver_name"`
	Designation   string `json:"approver_designation"`
	FileID        string `json:"file_id"`
	ClosureFileID string `json:"closure_file_id"`
	Remarks       string `json:"remarks"`
}
