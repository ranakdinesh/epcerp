package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type EngineerConfirmationsTimestamp = time.Time
type EngineerConfirmationsError = Error

func EngineerConfirmationsInvalid(f, m string) error { return &EngineerConfirmationsError{422, f, m} }

var EngineerConfirmationsUnauthorized = &EngineerConfirmationsError{401, "", "Authentication required"}
var EngineerConfirmationsForbidden = &EngineerConfirmationsError{403, "", "Current confirmation permission and Step 19 scope are required"}
var EngineerConfirmationsMissing = &EngineerConfirmationsError{404, "", "Engineer confirmation or current issued GRN not found"}
var EngineerConfirmationsStale = &EngineerConfirmationsError{409, "", "Confirmation or GRN evidence changed; retain your draft, reload and review"}
var engineerConfirmationsUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func EngineerConfirmationsID(s string) error {
	if !engineerConfirmationsUuid.MatchString(s) {
		return EngineerConfirmationsInvalid("id", "Use a valid identifier")
	}
	return nil
}
func EngineerConfirmationsText(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || required && s == "" {
		return s, EngineerConfirmationsInvalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func EngineerConfirmationsToday(t EngineerConfirmationsTimestamp) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}
func EngineerConfirmationsDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type EngineerConfirmationsAccess struct{ Read, Write, Scoped, Work bool }
type EngineerConfirmationsSourceFacts struct {
	GRNID              string `json:"grn_id"`
	GRNVersion         int32  `json:"grn_version"`
	GRNRevision        int32  `json:"grn_revision"`
	GRNNumber          string `json:"grn_number"`
	GRNDate            string `json:"grn_date"`
	Received           string `json:"received_qty"`
	Accepted           string `json:"accepted_qty"`
	Rejected           string `json:"rejected_qty"`
	WarrantyReference  string `json:"warranty_ref"`
	WarrantyValidUntil string `json:"warranty_valid_until"`
	ReceiptID          string `json:"receipt_id"`
	ReceiptVersion     int32  `json:"receipt_version"`
	ReceiptRevision    int32  `json:"receipt_revision"`
	ArrivalDate        string `json:"arrival_date"`
	ReceivingOfficer   string `json:"receiving_officer"`
	PackagesReceived   int32  `json:"packages_received"`
	PackagesCondition  string `json:"packages_condition"`
	TransitDamageNotes string `json:"transit_damage_notes"`
	GateEntryReference string `json:"gate_entry_ref"`
	DispatchID         string `json:"dispatch_id"`
	DispatchVersion    int32  `json:"dispatch_version"`
	DispatchRevision   int32  `json:"dispatch_revision"`
	DispatchDate       string `json:"dispatch_date"`
	LRNumber           string `json:"lr_number"`
	Transporter        string `json:"transporter_name"`
	ClearanceID        string `json:"clearance_id"`
	ClearanceVersion   int32  `json:"clearance_version"`
	ClearanceRevision  int32  `json:"clearance_revision"`
	Reference          string `json:"mdcc_ref"`
	ClearanceDate      string `json:"clearance_date"`
	ProjectID          string `json:"project_id"`
	PackageID          string `json:"package_id"`
	QualityID          string `json:"quality_id"`
	QualityVersion     int32  `json:"quality_version"`
	QualityRevision    int32  `json:"quality_revision"`
	ReportID           string `json:"report_id"`
	ReportVersion      int32  `json:"report_version"`
	ReportRevision     int32  `json:"report_revision"`
	IRNumber           string `json:"ir_number"`
	IRNNumber          string `json:"irn_number"`
	IssueDate          string `json:"issue_date"`
	OrderID            string `json:"order_id"`
	OrderVersion       int32  `json:"order_version"`
	Number             string `json:"number"`
	SupplierName       string `json:"supplier_name"`
	Description        string `json:"description"`
	Quantity           string `json:"cleared_quantity"`
	UOM                string `json:"uom"`
	InspectionID       string `json:"inspection_id"`
	InspectionVersion  int32  `json:"inspection_version"`
}
type EngineerConfirmationsSource struct {
	EngineerConfirmationsSourceFacts
	Available bool     `json:"available"`
	Blockers  []string `json:"blockers"`
}
type EngineerConfirmationsConfirmation struct {
	Status               string `json:"status"`
	Date                 string `json:"confirmation_date"`
	EngineerName         string `json:"engineer_name"`
	Designation          string `json:"designation"`
	MBookReference       string `json:"mbook_ref"`
	WorkPackageReference string `json:"work_package_ref"`
	Remarks              string `json:"remarks"`
}
type EngineerConfirmationsConfirmationInput = EngineerConfirmationsConfirmation
type EngineerConfirmationsRecord struct {
	ID           string                            `json:"id"`
	ProjectID    string                            `json:"project_id"`
	PackageID    string                            `json:"package_id"`
	Version      int32                             `json:"version"`
	Revision     int32                             `json:"revision"`
	Source       EngineerConfirmationsSource       `json:"source_snapshot"`
	Confirmation EngineerConfirmationsConfirmation `json:"confirmation"`
	Active       bool                              `json:"is_active"`
	CreatedAt    EngineerConfirmationsTimestamp    `json:"created_at"`
	UpdatedAt    EngineerConfirmationsTimestamp    `json:"updated_at"`
	CreatedBy    string                            `json:"created_by_id"`
	UpdatedBy    string                            `json:"updated_by_id"`
}
type EngineerConfirmationsCommand struct {
	Version      int32                                   `json:"version"`
	Action       string                                  `json:"action"`
	Reason       string                                  `json:"reason"`
	Confirmation *EngineerConfirmationsConfirmationInput `json:"confirmation,omitempty"`
}
type EngineerConfirmationsEvent struct {
	ID             string                         `json:"id"`
	ConfirmationID string                         `json:"confirmation_id"`
	Action         string                         `json:"action"`
	Reason         string                         `json:"reason"`
	Before         *EngineerConfirmationsRecord   `json:"before"`
	After          EngineerConfirmationsRecord    `json:"after"`
	At             EngineerConfirmationsTimestamp `json:"at"`
	ActorID        string                         `json:"actor_id"`
}
type EngineerConfirmationsWorkspace struct {
	Record   *EngineerConfirmationsRecord `json:"record"`
	Source   EngineerConfirmationsSource  `json:"source"`
	Actions  []string                     `json:"actions"`
	History  []EngineerConfirmationsEvent `json:"history"`
	TaskOnly bool                         `json:"task_only"`
}

func EngineerConfirmationsCopy(r EngineerConfirmationsRecord) EngineerConfirmationsRecord {
	r.Source.Blockers = append([]string{}, r.Source.Blockers...)
	return r
}
