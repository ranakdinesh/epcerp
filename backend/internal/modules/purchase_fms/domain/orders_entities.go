package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type OrdersTimestamp = time.Time
type OrdersError = Error

func OrdersInvalid(f, m string) error { return &OrdersError{422, f, m} }

var OrdersUnauthorized = &OrdersError{401, "", "Authentication required"}
var OrdersForbidden = &OrdersError{403, "", "Purchase-order access and assigned responsibility are required"}
var OrdersMissing = &OrdersError{404, "", "Purchase order or its source not found"}
var OrdersStale = &OrdersError{409, "", "Purchase order or source changed; retain your draft, reload and review"}
var ordersUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func OrdersID(s string) error {
	if !ordersUuid.MatchString(s) {
		return OrdersInvalid("id", "Use a valid identifier")
	}
	return nil
}
func OrdersText(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || (required && s == "") {
		return s, OrdersInvalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func OrdersToday(t time.Time) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}
func OrdersDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type OrdersAccess struct{ Read, Write, Release, Scoped, Work, RecordClient bool }
type OrdersSupplier struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	Address      string `json:"address"`
	Contact      string `json:"contact"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	TaxReference string `json:"tax_reference"`
	Registration string `json:"registration"`
}
type OrdersTechnical struct {
	CaseID               string `json:"case_id"`
	Version              int32  `json:"version"`
	Revision             int32  `json:"revision"`
	Reference            string `json:"reference"`
	ApprovalDate         string `json:"approval_date"`
	ApprovalFileID       string `json:"approval_file_id"`
	TransmittalReference string `json:"transmittal_reference"`
	QAPApproved          bool   `json:"qap_approved"`
	ConditionsClosed     bool   `json:"conditions_closed"`
}
type OrdersSourceFacts struct {
	ProjectID            string          `json:"project_id"`
	PackageID            string          `json:"package_id"`
	ProjectName          string          `json:"project_name"`
	Site                 string          `json:"site"`
	SourcingID           string          `json:"sourcing_id"`
	SourcingVersion      int32           `json:"sourcing_version"`
	LOIReference         string          `json:"loi_reference"`
	LOIDate              string          `json:"loi_date"`
	FinancialReference   string          `json:"financial_reference"`
	SupplierVerification string          `json:"supplier_verification"`
	Supplier             OrdersSupplier  `json:"supplier"`
	Technical            OrdersTechnical `json:"technical"`
	ItemID               string          `json:"item_id"`
	SKU                  string          `json:"sku"`
	Description          string          `json:"description"`
	UOM                  string          `json:"uom"`
	Quantity             string          `json:"quantity"`
	Currency             string          `json:"currency"`
	PricingMode          string          `json:"pricing_mode"`
	UnitPrice            string          `json:"unit_price"`
	Base                 string          `json:"base"`
	Discount             string          `json:"discount"`
	Tax                  string          `json:"tax"`
	Freight              string          `json:"freight"`
	Other                string          `json:"other"`
	Total                string          `json:"total"`
	DeliveryDays         int             `json:"delivery_days"`
	PaymentTerms         string          `json:"payment_terms"`
	Warranty             string          `json:"warranty"`
	LOITerms             string          `json:"loi_terms"`
}
type OrdersSource struct {
	OrdersSourceFacts
	Available bool     `json:"available"`
	Blockers  []string `json:"blockers"`
}
type OrdersDetails struct {
	Date               string `json:"date"`
	IssuerName         string `json:"issuer_name"`
	IssuerAddress      string `json:"issuer_address"`
	IssuerTaxReference string `json:"issuer_tax_reference"`
	IssuerRegistration string `json:"issuer_registration"`
	IssuerEmail        string `json:"issuer_email"`
	BillTo             string `json:"bill_to"`
	DeliveryAddress    string `json:"delivery_address"`
}
type OrdersClientSubmission struct {
	Reference string `json:"reference"`
	Date      string `json:"date"`
}
type OrdersClientDecision struct {
	Status      string `json:"status"`
	Reference   string `json:"reference"`
	Date        string `json:"date"`
	Signatory   string `json:"signatory"`
	Designation string `json:"designation"`
	FileID      string `json:"file_id"`
}
type OrdersClientApproval struct {
	Status        string                  `json:"status"`
	DraftRevision int32                   `json:"draft_revision"`
	Submission    *OrdersClientSubmission `json:"submission,omitempty"`
	Decision      *OrdersClientDecision   `json:"decision,omitempty"`
	SubmittedBy   string                  `json:"submitted_by_id"`
	RecordedBy    string                  `json:"recorded_by_id"`
	RecordedAt    *OrdersTimestamp        `json:"recorded_at,omitempty"`
}
type OrdersOrder struct {
	DraftRevision  int32                `json:"draft_revision"`
	ClientApproval OrdersClientApproval `json:"client_approval"`
	ID             string               `json:"id"`
	ProjectID      string               `json:"project_id"`
	PackageID      string               `json:"package_id"`
	Version        int32                `json:"version"`
	State          string               `json:"state"`
	Number         string               `json:"number"`
	Source         OrdersSource         `json:"source_snapshot"`
	Details        OrdersDetails        `json:"details"`
	ReleasedAt     *OrdersTimestamp     `json:"released_at"`
	ReleasedBy     string               `json:"released_by_id"`
	Active         bool                 `json:"is_active"`
	CreatedAt      OrdersTimestamp      `json:"created_at"`
	UpdatedAt      OrdersTimestamp      `json:"updated_at"`
	CreatedBy      string               `json:"created_by_id"`
	UpdatedBy      string               `json:"updated_by_id"`
}
type OrdersCommand struct {
	Version        int32                   `json:"version"`
	Action         string                  `json:"action"`
	Reason         string                  `json:"reason"`
	Details        *OrdersDetails          `json:"details,omitempty"`
	Submission     *OrdersClientSubmission `json:"submission,omitempty"`
	ClientDecision *OrdersClientDecision   `json:"client_decision,omitempty"`
}
type OrdersEvent struct {
	ID      string          `json:"id"`
	OrderID string          `json:"order_id"`
	Action  string          `json:"action"`
	Reason  string          `json:"reason"`
	Before  *OrdersOrder    `json:"before"`
	After   OrdersOrder     `json:"after"`
	At      OrdersTimestamp `json:"at"`
	ActorID string          `json:"actor_id"`
}
type OrdersWorkspace struct {
	Order    *OrdersOrder  `json:"order"`
	Source   OrdersSource  `json:"source"`
	Actions  []string      `json:"actions"`
	History  []OrdersEvent `json:"history"`
	TaskOnly bool          `json:"task_only"`
}

func OrdersCopy(v OrdersOrder) OrdersOrder {
	v.Source.Blockers = append([]string{}, v.Source.Blockers...)
	if v.ClientApproval.Submission != nil {
		d := *v.ClientApproval.Submission
		v.ClientApproval.Submission = &d
	}
	if v.ClientApproval.Decision != nil {
		d := *v.ClientApproval.Decision
		v.ClientApproval.Decision = &d
	}
	if v.ClientApproval.RecordedAt != nil {
		t := *v.ClientApproval.RecordedAt
		v.ClientApproval.RecordedAt = &t
	}
	if v.ReleasedAt != nil {
		t := *v.ReleasedAt
		v.ReleasedAt = &t
	}
	return v
}

// DocumentEvent records a requested export, not proof of printing or vendor acceptance.
type OrdersDocumentEvent struct {
	ID, OrderID, ProjectID, ActorID string
	Version                         int32
	At                              OrdersTimestamp
}
