package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type DispatchTimestamp = time.Time
type DispatchError = Error

func DispatchInvalid(f, m string) error { return &DispatchError{422, f, m} }

var DispatchUnauthorized = &DispatchError{401, "", "Authentication required"}
var DispatchForbidden = &DispatchError{403, "", "Current dispatch permission and Step 16 scope are required"}
var DispatchMissing = &DispatchError{404, "", "Dispatch or current client MDCC not found"}
var DispatchStale = &DispatchError{409, "", "Dispatch or client clearance evidence changed; retain your draft, reload and review"}
var dispatchUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func DispatchID(s string) error {
	if !dispatchUuid.MatchString(s) {
		return DispatchInvalid("id", "Use a valid identifier")
	}
	return nil
}
func DispatchText(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || required && s == "" {
		return s, DispatchInvalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func DispatchToday(t DispatchTimestamp) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}
func DispatchDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type DispatchAccess struct{ Read, Write, Scoped, Work bool }
type DispatchSourceFacts struct {
	ClearanceID       string `json:"clearance_id"`
	ClearanceVersion  int32  `json:"clearance_version"`
	ClearanceRevision int32  `json:"clearance_revision"`
	Reference         string `json:"mdcc_ref"`
	ClearanceDate     string `json:"clearance_date"`
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
type DispatchSource struct {
	DispatchSourceFacts
	Available bool     `json:"available"`
	Blockers  []string `json:"blockers"`
}
type Dispatch struct {
	Status            string `json:"status"`
	Date              string `json:"dispatch_date"`
	Transporter       string `json:"transporter_name"`
	LRNumber          string `json:"lr_number"`
	LRDate            string `json:"lr_date"`
	Vehicle           string `json:"vehicle_number"`
	EwayNumber        string `json:"eway_bill_number"`
	EwayValidUntil    string `json:"eway_bill_valid_until"`
	InvoiceNumber     string `json:"invoice_number"`
	InvoiceDate       string `json:"invoice_date"`
	DriverContact     string `json:"driver_contact"`
	DocumentReference string `json:"file_name"`
}
type DispatchInput = Dispatch
type DispatchRecord struct {
	ID        string            `json:"id"`
	ProjectID string            `json:"project_id"`
	PackageID string            `json:"package_id"`
	Version   int32             `json:"version"`
	Revision  int32             `json:"revision"`
	Source    DispatchSource    `json:"source_snapshot"`
	Dispatch  Dispatch          `json:"dispatch"`
	Active    bool              `json:"is_active"`
	CreatedAt DispatchTimestamp `json:"created_at"`
	UpdatedAt DispatchTimestamp `json:"updated_at"`
	CreatedBy string            `json:"created_by_id"`
	UpdatedBy string            `json:"updated_by_id"`
}
type DispatchCommand struct {
	Version  int32          `json:"version"`
	Action   string         `json:"action"`
	Reason   string         `json:"reason"`
	Dispatch *DispatchInput `json:"dispatch,omitempty"`
}
type DispatchEvent struct {
	ID         string            `json:"id"`
	DispatchID string            `json:"dispatch_id"`
	Action     string            `json:"action"`
	Reason     string            `json:"reason"`
	Before     *DispatchRecord   `json:"before"`
	After      DispatchRecord    `json:"after"`
	At         DispatchTimestamp `json:"at"`
	ActorID    string            `json:"actor_id"`
}
type DispatchWorkspace struct {
	Record   *DispatchRecord `json:"record"`
	Source   DispatchSource  `json:"source"`
	Actions  []string        `json:"actions"`
	History  []DispatchEvent `json:"history"`
	TaskOnly bool            `json:"task_only"`
}

func DispatchCopy(r DispatchRecord) DispatchRecord {
	r.Source.Blockers = append([]string{}, r.Source.Blockers...)
	return r
}
