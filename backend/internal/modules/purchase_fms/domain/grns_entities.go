package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type GRNsTimestamp = time.Time
type GRNsError = Error

func GRNsInvalid(f, m string) error { return &GRNsError{422, f, m} }

var GRNsUnauthorized = &GRNsError{401, "", "Authentication required"}
var GRNsForbidden = &GRNsError{403, "", "Current grn permission and Step 18 scope are required"}
var GRNsMissing = &GRNsError{404, "", "GRN or current physical site receipt not found"}
var GRNsStale = &GRNsError{409, "", "GRN or site receipt evidence changed; retain your draft, reload and review"}
var gRNsUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func GRNsID(s string) error {
	if !gRNsUuid.MatchString(s) {
		return GRNsInvalid("id", "Use a valid identifier")
	}
	return nil
}
func GRNsText(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || required && s == "" {
		return s, GRNsInvalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func GRNsToday(t GRNsTimestamp) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}
func GRNsDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type GRNsAccess struct{ Read, Write, Scoped, Work bool }
type GRNsSourceFacts struct {
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
type GRNsSource struct {
	GRNsSourceFacts
	Available bool     `json:"available"`
	Blockers  []string `json:"blockers"`
}
type GRNsGRNInput struct {
	Status             string `json:"status"`
	Number             string `json:"grn_number"`
	Date               string `json:"grn_date"`
	Received           string `json:"received_qty"`
	Accepted           string `json:"accepted_qty"`
	Rejected           string `json:"rejected_qty"`
	StoreKeeper        string `json:"store_keeper_name"`
	Remarks            string `json:"remarks"`
	WarrantyReference  string `json:"warranty_ref"`
	WarrantyValidUntil string `json:"warranty_valid_until"`
	DocumentReference  string `json:"file_name"`
}
type GRNsGRN struct {
	GRNsGRNInput
	UOM string `json:"uom"`
}
type GRNsRecord struct {
	ID        string        `json:"id"`
	ProjectID string        `json:"project_id"`
	PackageID string        `json:"package_id"`
	Version   int32         `json:"version"`
	Revision  int32         `json:"revision"`
	Source    GRNsSource    `json:"source_snapshot"`
	GRN       GRNsGRN       `json:"grn"`
	Active    bool          `json:"is_active"`
	CreatedAt GRNsTimestamp `json:"created_at"`
	UpdatedAt GRNsTimestamp `json:"updated_at"`
	CreatedBy string        `json:"created_by_id"`
	UpdatedBy string        `json:"updated_by_id"`
}
type GRNsCommand struct {
	Version int32         `json:"version"`
	Action  string        `json:"action"`
	Reason  string        `json:"reason"`
	GRN     *GRNsGRNInput `json:"grn,omitempty"`
}
type GRNsEvent struct {
	ID      string        `json:"id"`
	GRNID   string        `json:"grn_id"`
	Action  string        `json:"action"`
	Reason  string        `json:"reason"`
	Before  *GRNsRecord   `json:"before"`
	After   GRNsRecord    `json:"after"`
	At      GRNsTimestamp `json:"at"`
	ActorID string        `json:"actor_id"`
}
type GRNsWorkspace struct {
	Record   *GRNsRecord `json:"record"`
	Source   GRNsSource  `json:"source"`
	Actions  []string    `json:"actions"`
	History  []GRNsEvent `json:"history"`
	TaskOnly bool        `json:"task_only"`
}

func GRNsCopy(r GRNsRecord) GRNsRecord {
	r.Source.Blockers = append([]string{}, r.Source.Blockers...)
	return r
}
