package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type InspectionReportsTimestamp = time.Time
type InspectionReportsError = Error

func InspectionReportsInvalid(f, m string) error { return &InspectionReportsError{422, f, m} }

var InspectionReportsUnauthorized = &InspectionReportsError{401, "", "Authentication required"}
var InspectionReportsForbidden = &InspectionReportsError{403, "", "Current report permission and Step 13 scope are required"}
var InspectionReportsMissing = &InspectionReportsError{404, "", "Report or current cleared inspection not found"}
var InspectionReportsStale = &InspectionReportsError{409, "", "Report or inspection evidence changed; retain your draft, reload and review"}
var inspectionReportsUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func InspectionReportsID(s string) error {
	if !inspectionReportsUuid.MatchString(s) {
		return InspectionReportsInvalid("id", "Use a valid identifier")
	}
	return nil
}
func InspectionReportsText(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || required && s == "" {
		return s, InspectionReportsInvalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func InspectionReportsToday(t InspectionReportsTimestamp) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}
func InspectionReportsDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type InspectionReportsAccess struct{ Read, Write, Scoped, Work bool }
type InspectionReportsSourceFacts struct {
	ProjectID          string `json:"project_id"`
	PackageID          string `json:"package_id"`
	InspectionID       string `json:"inspection_id"`
	InspectionVersion  int32  `json:"inspection_version"`
	InspectionRevision int32  `json:"inspection_revision"`
	ActualDate         string `json:"actual_date"`
	OrderID            string `json:"order_id"`
	OrderVersion       int32  `json:"order_version"`
	Number             string `json:"number"`
	SupplierName       string `json:"supplier_name"`
	Description        string `json:"description"`
	Quantity           string `json:"quantity"`
	UOM                string `json:"uom"`
	QAPReference       string `json:"qap_reference"`
}
type InspectionReportsSource struct {
	InspectionReportsSourceFacts
	Available bool     `json:"available"`
	Blockers  []string `json:"blockers"`
}
type InspectionReportsReport struct {
	Status            string `json:"status"`
	IRNumber          string `json:"ir_number"`
	IRNNumber         string `json:"irn_number"`
	IssueDate         string `json:"issue_date"`
	Authority         string `json:"issuing_authority"`
	Quantity          string `json:"cleared_quantity"`
	UOM               string `json:"uom"`
	DocumentReference string `json:"file_name"`
}
type InspectionReportsReportInput struct {
	Status            string `json:"status"`
	IRNumber          string `json:"ir_number"`
	IRNNumber         string `json:"irn_number"`
	IssueDate         string `json:"issue_date"`
	Authority         string `json:"issuing_authority"`
	Quantity          string `json:"cleared_quantity"`
	DocumentReference string `json:"file_name"`
}
type InspectionReportsRecord struct {
	ID        string                     `json:"id"`
	ProjectID string                     `json:"project_id"`
	PackageID string                     `json:"package_id"`
	Version   int32                      `json:"version"`
	Revision  int32                      `json:"revision"`
	Source    InspectionReportsSource    `json:"source_snapshot"`
	Report    InspectionReportsReport    `json:"report"`
	Active    bool                       `json:"is_active"`
	CreatedAt InspectionReportsTimestamp `json:"created_at"`
	UpdatedAt InspectionReportsTimestamp `json:"updated_at"`
	CreatedBy string                     `json:"created_by_id"`
	UpdatedBy string                     `json:"updated_by_id"`
}
type InspectionReportsCommand struct {
	Version int32                         `json:"version"`
	Action  string                        `json:"action"`
	Reason  string                        `json:"reason"`
	Report  *InspectionReportsReportInput `json:"report,omitempty"`
}
type InspectionReportsEvent struct {
	ID       string                     `json:"id"`
	ReportID string                     `json:"report_id"`
	Action   string                     `json:"action"`
	Reason   string                     `json:"reason"`
	Before   *InspectionReportsRecord   `json:"before"`
	After    InspectionReportsRecord    `json:"after"`
	At       InspectionReportsTimestamp `json:"at"`
	ActorID  string                     `json:"actor_id"`
}
type InspectionReportsWorkspace struct {
	Record   *InspectionReportsRecord `json:"record"`
	Source   InspectionReportsSource  `json:"source"`
	Actions  []string                 `json:"actions"`
	History  []InspectionReportsEvent `json:"history"`
	TaskOnly bool                     `json:"task_only"`
}

func InspectionReportsCopy(r InspectionReportsRecord) InspectionReportsRecord {
	r.Source.Blockers = append([]string{}, r.Source.Blockers...)
	return r
}
