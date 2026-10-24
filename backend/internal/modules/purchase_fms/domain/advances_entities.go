package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type AdvancesTimestamp = time.Time
type AdvancesError = Error

func AdvancesInvalid(f, m string) error { return &AdvancesError{422, f, m} }

var AdvancesUnauthorized = &AdvancesError{401, "", "Authentication required"}
var AdvancesForbidden = &AdvancesError{403, "", "Current advance-request permission and scope are required"}
var AdvancesMissing = &AdvancesError{404, "", "Advance request or its released PO not found"}
var AdvancesStale = &AdvancesError{409, "", "Advance request or its approved sources changed; retain your draft, reload and review"}
var advancesUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func AdvancesID(s string) error {
	if !advancesUuid.MatchString(s) {
		return AdvancesInvalid("id", "Use a valid identifier")
	}
	return nil
}
func AdvancesText(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || (required && s == "") {
		return s, AdvancesInvalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func AdvancesToday(t time.Time) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}
func AdvancesDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type AdvancesAccess struct{ Read, Write, Work, Review, Disburse, Scoped bool }
type AdvancesSourceFacts struct {
	ProjectID            string `json:"project_id"`
	PackageID            string `json:"package_id"`
	OrderID              string `json:"order_id"`
	OrderVersion         int32  `json:"order_version"`
	Number               string `json:"number"`
	Date                 string `json:"date"`
	SupplierID           string `json:"supplier_id"`
	SupplierName         string `json:"supplier_name"`
	Currency             string `json:"currency"`
	Total                string `json:"total"`
	PaymentTerms         string `json:"payment_terms"`
	AcceptanceID         string `json:"acceptance_id"`
	AcceptanceVersion    int32  `json:"acceptance_version"`
	SupplierVerification string `json:"supplier_verification"`
	CalculationBasis     string `json:"calculation_basis"`
}
type AdvancesSource struct {
	AdvancesSourceFacts
	Available bool     `json:"available"`
	Blockers  []string `json:"blockers"`
}
type AdvancesDetails struct {
	Kind             string `json:"kind"`
	RequestNo        string `json:"request_no"`
	RequestDate      string `json:"request_date"`
	Percentage       string `json:"advance_pct"`
	InvoiceReference string `json:"proforma_invoice_ref"`
	InvoiceDate      string `json:"proforma_invoice_date"`
	BankReference    string `json:"bank_account_ref"`
	Notes            string `json:"notes"`
}
type AdvancesPayment struct {
	Voucher string `json:"voucher_ref"`
	Date    string `json:"disbursed_date"`
	Notes   string `json:"notes"`
}
type AdvancesRecord struct {
	ID               string             `json:"id"`
	ProjectID        string             `json:"project_id"`
	PackageID        string             `json:"package_id"`
	Version          int32              `json:"version"`
	State            string             `json:"state"`
	Source           AdvancesSource     `json:"source_snapshot"`
	Details          AdvancesDetails    `json:"details"`
	Amount           string             `json:"amount"`
	SubmittedBy      string             `json:"submitted_by_id"`
	SubmittedAt      *AdvancesTimestamp `json:"submitted_at"`
	SubmittedVersion int32              `json:"submitted_version"`
	DecisionID       string             `json:"decision_id"`
	ReviewedBy       string             `json:"reviewed_by_id"`
	ReviewedAt       *AdvancesTimestamp `json:"reviewed_at"`
	Payment          *AdvancesPayment   `json:"payment"`
	Active           bool               `json:"is_active"`
	CreatedAt        AdvancesTimestamp  `json:"created_at"`
	UpdatedAt        AdvancesTimestamp  `json:"updated_at"`
	CreatedBy        string             `json:"created_by_id"`
	UpdatedBy        string             `json:"updated_by_id"`
}
type AdvancesCommand struct {
	Version int32            `json:"version"`
	Action  string           `json:"action"`
	Reason  string           `json:"reason"`
	Details *AdvancesDetails `json:"details,omitempty"`
	Payment *AdvancesPayment `json:"payment,omitempty"`
}
type AdvancesEvent struct {
	ID        string            `json:"id"`
	RequestID string            `json:"request_id"`
	Action    string            `json:"action"`
	Reason    string            `json:"reason"`
	Before    *AdvancesRecord   `json:"before"`
	After     AdvancesRecord    `json:"after"`
	At        AdvancesTimestamp `json:"at"`
	ActorID   string            `json:"actor_id"`
}
type AdvancesWorkspace struct {
	Record   *AdvancesRecord `json:"record"`
	Source   AdvancesSource  `json:"source"`
	Actions  []string        `json:"actions"`
	History  []AdvancesEvent `json:"history"`
	TaskOnly bool            `json:"task_only"`
}

func AdvancesCopy(r AdvancesRecord) AdvancesRecord {
	r.Source.Blockers = append([]string{}, r.Source.Blockers...)
	if r.SubmittedAt != nil {
		v := *r.SubmittedAt
		r.SubmittedAt = &v
	}
	if r.ReviewedAt != nil {
		v := *r.ReviewedAt
		r.ReviewedAt = &v
	}
	if r.Payment != nil {
		v := *r.Payment
		r.Payment = &v
	}
	return r
}

type AdvancesQueueRow struct {
	ID            string            `json:"id"`
	ProjectID     string            `json:"project_id"`
	PackageID     string            `json:"package_id"`
	ProjectName   string            `json:"project_name"`
	ProjectNumber string            `json:"project_number"`
	Number        string            `json:"po_number"`
	RequestNo     string            `json:"request_no"`
	State         string            `json:"state"`
	Amount        string            `json:"amount"`
	Currency      string            `json:"currency"`
	UpdatedAt     AdvancesTimestamp `json:"updated_at"`
}
type AdvancesQueuePage struct {
	Rows []AdvancesQueueRow `json:"rows"`
	Next *int32             `json:"next_offset"`
}
