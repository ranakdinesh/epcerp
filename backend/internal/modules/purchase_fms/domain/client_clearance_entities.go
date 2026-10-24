package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type ClientClearanceTimestamp = time.Time
type ClientClearanceError = Error

func ClientClearanceInvalid(f, m string) error { return &ClientClearanceError{422, f, m} }

var ClientClearanceUnauthorized = &ClientClearanceError{401, "", "Authentication required"}
var ClientClearanceForbidden = &ClientClearanceError{403, "", "Current clearance permission and Step 15 scope are required"}
var ClientClearanceMissing = &ClientClearanceError{404, "", "Clearance or current cleared inspection not found"}
var ClientClearanceStale = &ClientClearanceError{409, "", "Clearance or inspection evidence changed; retain your draft, reload and review"}
var clientClearanceUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func ClientClearanceID(s string) error {
	if !clientClearanceUuid.MatchString(s) {
		return ClientClearanceInvalid("id", "Use a valid identifier")
	}
	return nil
}
func ClientClearanceText(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || required && s == "" {
		return s, ClientClearanceInvalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func ClientClearanceToday(t ClientClearanceTimestamp) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}
func ClientClearanceDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type ClientClearanceAccess struct{ Read, Write, Scoped, Work bool }
type ClientClearanceSourceFacts struct {
	ProjectID         string `json:"project_id"`
	PackageID         string `json:"package_id"`
	QualityID         string `json:"quality_id"`
	QualityVersion    int32  `json:"quality_version"`
	QualityRevision   int32  `json:"quality_revision"`
	ReportID          string `json:"report_id"`
	ReportVersion     int32  `json:"report_version"`
	ReportRevision    int32  `json:"report_revision"`
	IRNumber          string `json:"ir_number"`
	IRNNumber         string `json:"irn_number"`
	IssueDate         string `json:"issue_date"`
	OrderID           string `json:"order_id"`
	OrderVersion      int32  `json:"order_version"`
	Number            string `json:"number"`
	SupplierName      string `json:"supplier_name"`
	Description       string `json:"description"`
	Quantity          string `json:"cleared_quantity"`
	UOM               string `json:"uom"`
	InspectionID      string `json:"inspection_id"`
	InspectionVersion int32  `json:"inspection_version"`
}
type ClientClearanceSource struct {
	ClientClearanceSourceFacts
	Available bool     `json:"available"`
	Blockers  []string `json:"blockers"`
}
type ClientClearanceClearance struct {
	Status            string `json:"status"`
	Reference         string `json:"mdcc_ref"`
	Date              string `json:"clearance_date"`
	Authority         string `json:"cleared_by_authority"`
	Remarks           string `json:"remarks"`
	DocumentReference string `json:"file_name"`
}
type ClientClearanceClearanceInput = ClientClearanceClearance
type ClientClearanceRecord struct {
	ID        string                   `json:"id"`
	ProjectID string                   `json:"project_id"`
	PackageID string                   `json:"package_id"`
	Version   int32                    `json:"version"`
	Revision  int32                    `json:"revision"`
	Source    ClientClearanceSource    `json:"source_snapshot"`
	Clearance ClientClearanceClearance `json:"clearance"`
	Active    bool                     `json:"is_active"`
	CreatedAt ClientClearanceTimestamp `json:"created_at"`
	UpdatedAt ClientClearanceTimestamp `json:"updated_at"`
	CreatedBy string                   `json:"created_by_id"`
	UpdatedBy string                   `json:"updated_by_id"`
}
type ClientClearanceCommand struct {
	Version   int32                          `json:"version"`
	Action    string                         `json:"action"`
	Reason    string                         `json:"reason"`
	Clearance *ClientClearanceClearanceInput `json:"clearance,omitempty"`
}
type ClientClearanceEvent struct {
	ID          string                   `json:"id"`
	ClearanceID string                   `json:"clearance_id"`
	Action      string                   `json:"action"`
	Reason      string                   `json:"reason"`
	Before      *ClientClearanceRecord   `json:"before"`
	After       ClientClearanceRecord    `json:"after"`
	At          ClientClearanceTimestamp `json:"at"`
	ActorID     string                   `json:"actor_id"`
}
type ClientClearanceWorkspace struct {
	Record   *ClientClearanceRecord `json:"record"`
	Source   ClientClearanceSource  `json:"source"`
	Actions  []string               `json:"actions"`
	History  []ClientClearanceEvent `json:"history"`
	TaskOnly bool                   `json:"task_only"`
}

func ClientClearanceCopy(r ClientClearanceRecord) ClientClearanceRecord {
	r.Source.Blockers = append([]string{}, r.Source.Blockers...)
	return r
}
