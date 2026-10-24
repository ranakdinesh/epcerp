package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type Timestamp = time.Time
type Error struct {
	Status         int
	Field, Message string
}

func (e *Error) Error() string  { return e.Message }
func Invalid(f, m string) error { return &Error{422, f, m} }

var Unauthorized = &Error{401, "", "Authentication required"}
var Forbidden = &Error{403, "", "Current acceptance permission and Step 20 scope are required"}
var Missing = &Error{404, "", "Accounts acceptance or current issued GRN not found"}
var Stale = &Error{409, "", "Acceptance or GRN evidence changed; retain your draft, reload and review"}
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func ID(s string) error {
	if !uuid.MatchString(s) {
		return Invalid("id", "Use a valid identifier")
	}
	return nil
}
func Text(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || required && s == "" {
		return s, Invalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func Today(t Timestamp) string { return t.In(time.FixedZone("India", 19800)).Format("2006-01-02") }
func Date(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type Actor struct {
	ID                string
	Active, Superuser bool
	Permissions       []string
}
type Access struct{ Read, Write, Scoped, Work bool }
type SourceFacts struct {
	EngineerID           string `json:"engineer_id"`
	EngineerVersion      int32  `json:"engineer_version"`
	EngineerRevision     int32  `json:"engineer_revision"`
	ConfirmationDate     string `json:"confirmation_date"`
	EngineerName         string `json:"engineer_name"`
	Designation          string `json:"designation"`
	MBookReference       string `json:"mbook_ref"`
	WorkPackageReference string `json:"work_package_ref"`
	EngineerRemarks      string `json:"engineer_remarks"`
	Currency             string `json:"currency"`
	GRNID                string `json:"grn_id"`
	GRNVersion           int32  `json:"grn_version"`
	GRNRevision          int32  `json:"grn_revision"`
	GRNNumber            string `json:"grn_number"`
	GRNDate              string `json:"grn_date"`
	Received             string `json:"received_qty"`
	Accepted             string `json:"accepted_qty"`
	Rejected             string `json:"rejected_qty"`
	WarrantyReference    string `json:"warranty_ref"`
	WarrantyValidUntil   string `json:"warranty_valid_until"`
	ReceiptID            string `json:"receipt_id"`
	ReceiptVersion       int32  `json:"receipt_version"`
	ReceiptRevision      int32  `json:"receipt_revision"`
	ArrivalDate          string `json:"arrival_date"`
	ReceivingOfficer     string `json:"receiving_officer"`
	PackagesReceived     int32  `json:"packages_received"`
	PackagesCondition    string `json:"packages_condition"`
	TransitDamageNotes   string `json:"transit_damage_notes"`
	GateEntryReference   string `json:"gate_entry_ref"`
	DispatchID           string `json:"dispatch_id"`
	DispatchVersion      int32  `json:"dispatch_version"`
	DispatchRevision     int32  `json:"dispatch_revision"`
	DispatchDate         string `json:"dispatch_date"`
	LRNumber             string `json:"lr_number"`
	Transporter          string `json:"transporter_name"`
	ClearanceID          string `json:"clearance_id"`
	ClearanceVersion     int32  `json:"clearance_version"`
	ClearanceRevision    int32  `json:"clearance_revision"`
	Reference            string `json:"mdcc_ref"`
	ClearanceDate        string `json:"clearance_date"`
	ProjectID            string `json:"project_id"`
	PackageID            string `json:"package_id"`
	QualityID            string `json:"quality_id"`
	QualityVersion       int32  `json:"quality_version"`
	QualityRevision      int32  `json:"quality_revision"`
	ReportID             string `json:"report_id"`
	ReportVersion        int32  `json:"report_version"`
	ReportRevision       int32  `json:"report_revision"`
	IRNumber             string `json:"ir_number"`
	IRNNumber            string `json:"irn_number"`
	IssueDate            string `json:"issue_date"`
	OrderID              string `json:"order_id"`
	OrderVersion         int32  `json:"order_version"`
	Number               string `json:"number"`
	SupplierName         string `json:"supplier_name"`
	Description          string `json:"description"`
	Quantity             string `json:"cleared_quantity"`
	UOM                  string `json:"uom"`
	InspectionID         string `json:"inspection_id"`
	InspectionVersion    int32  `json:"inspection_version"`
}
type Source struct {
	SourceFacts
	Available bool     `json:"available"`
	Blockers  []string `json:"blockers"`
}
type Acceptance struct {
	Status           string `json:"status"`
	Date             string `json:"acceptance_date"`
	OfficerName      string `json:"officer_name"`
	VoucherReference string `json:"voucher_ref"`
	BookedAmount     string `json:"booked_amount"`
	Notes            string `json:"notes"`
}
type AcceptanceInput = Acceptance
type Record struct {
	ID         string     `json:"id"`
	ProjectID  string     `json:"project_id"`
	PackageID  string     `json:"package_id"`
	Version    int32      `json:"version"`
	Revision   int32      `json:"revision"`
	Source     Source     `json:"source_snapshot"`
	Acceptance Acceptance `json:"acceptance"`
	Active     bool       `json:"is_active"`
	CreatedAt  Timestamp  `json:"created_at"`
	UpdatedAt  Timestamp  `json:"updated_at"`
	CreatedBy  string     `json:"created_by_id"`
	UpdatedBy  string     `json:"updated_by_id"`
}
type Command struct {
	Version    int32            `json:"version"`
	Action     string           `json:"action"`
	Reason     string           `json:"reason"`
	Acceptance *AcceptanceInput `json:"acceptance,omitempty"`
}
type Event struct {
	ID           string    `json:"id"`
	AcceptanceID string    `json:"acceptance_id"`
	Action       string    `json:"action"`
	Reason       string    `json:"reason"`
	Before       *Record   `json:"before"`
	After        Record    `json:"after"`
	At           Timestamp `json:"at"`
	ActorID      string    `json:"actor_id"`
}
type Workspace struct {
	Record   *Record  `json:"record"`
	Source   Source   `json:"source"`
	Actions  []string `json:"actions"`
	History  []Event  `json:"history"`
	TaskOnly bool     `json:"task_only"`
}

func Copy(r Record) Record { r.Source.Blockers = append([]string{}, r.Source.Blockers...); return r }
