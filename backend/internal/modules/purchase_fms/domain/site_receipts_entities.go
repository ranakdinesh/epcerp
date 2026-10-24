package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type SiteReceiptsTimestamp = time.Time
type SiteReceiptsError = Error

func SiteReceiptsInvalid(f, m string) error { return &SiteReceiptsError{422, f, m} }

var SiteReceiptsUnauthorized = &SiteReceiptsError{401, "", "Authentication required"}
var SiteReceiptsForbidden = &SiteReceiptsError{403, "", "Current receipt permission and Step 17 scope are required"}
var SiteReceiptsMissing = &SiteReceiptsError{404, "", "Receipt or current material dispatch not found"}
var SiteReceiptsStale = &SiteReceiptsError{409, "", "Receipt or dispatch evidence changed; retain your draft, reload and review"}
var siteReceiptsUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func SiteReceiptsID(s string) error {
	if !siteReceiptsUuid.MatchString(s) {
		return SiteReceiptsInvalid("id", "Use a valid identifier")
	}
	return nil
}
func SiteReceiptsText(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || required && s == "" {
		return s, SiteReceiptsInvalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func SiteReceiptsToday(t SiteReceiptsTimestamp) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}
func SiteReceiptsDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type SiteReceiptsAccess struct{ Read, Write, Scoped, Work bool }
type SiteReceiptsSourceFacts struct {
	DispatchID        string `json:"dispatch_id"`
	DispatchVersion   int32  `json:"dispatch_version"`
	DispatchRevision  int32  `json:"dispatch_revision"`
	DispatchDate      string `json:"dispatch_date"`
	LRNumber          string `json:"lr_number"`
	Transporter       string `json:"transporter_name"`
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
type SiteReceiptsSource struct {
	SiteReceiptsSourceFacts
	Available bool     `json:"available"`
	Blockers  []string `json:"blockers"`
}
type SiteReceiptsReceipt struct {
	Status             string `json:"status"`
	ArrivalDate        string `json:"arrival_date"`
	ReceivingOfficer   string `json:"receiving_officer"`
	PackagesReceived   int32  `json:"packages_received"`
	PackagesCondition  string `json:"packages_condition"`
	TransitDamageNotes string `json:"transit_damage_notes"`
	GateEntryReference string `json:"gate_entry_ref"`
}
type SiteReceiptsReceiptInput = SiteReceiptsReceipt
type SiteReceiptsRecord struct {
	ID        string                `json:"id"`
	ProjectID string                `json:"project_id"`
	PackageID string                `json:"package_id"`
	Version   int32                 `json:"version"`
	Revision  int32                 `json:"revision"`
	Source    SiteReceiptsSource    `json:"source_snapshot"`
	Receipt   SiteReceiptsReceipt   `json:"receipt"`
	Active    bool                  `json:"is_active"`
	CreatedAt SiteReceiptsTimestamp `json:"created_at"`
	UpdatedAt SiteReceiptsTimestamp `json:"updated_at"`
	CreatedBy string                `json:"created_by_id"`
	UpdatedBy string                `json:"updated_by_id"`
}
type SiteReceiptsCommand struct {
	Version int32                     `json:"version"`
	Action  string                    `json:"action"`
	Reason  string                    `json:"reason"`
	Receipt *SiteReceiptsReceiptInput `json:"receipt,omitempty"`
}
type SiteReceiptsEvent struct {
	ID        string                `json:"id"`
	ReceiptID string                `json:"receipt_id"`
	Action    string                `json:"action"`
	Reason    string                `json:"reason"`
	Before    *SiteReceiptsRecord   `json:"before"`
	After     SiteReceiptsRecord    `json:"after"`
	At        SiteReceiptsTimestamp `json:"at"`
	ActorID   string                `json:"actor_id"`
}
type SiteReceiptsWorkspace struct {
	Record   *SiteReceiptsRecord `json:"record"`
	Source   SiteReceiptsSource  `json:"source"`
	Actions  []string            `json:"actions"`
	History  []SiteReceiptsEvent `json:"history"`
	TaskOnly bool                `json:"task_only"`
}

func SiteReceiptsCopy(r SiteReceiptsRecord) SiteReceiptsRecord {
	r.Source.Blockers = append([]string{}, r.Source.Blockers...)
	return r
}
