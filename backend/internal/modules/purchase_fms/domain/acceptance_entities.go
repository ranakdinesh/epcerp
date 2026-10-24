package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type AcceptanceTimestamp = time.Time
type AcceptanceError = Error

func AcceptanceInvalid(f, m string) error { return &AcceptanceError{422, f, m} }

var AcceptanceUnauthorized = &AcceptanceError{401, "", "Authentication required"}
var AcceptanceForbidden = &AcceptanceError{403, "", "Current vendor-acceptance permission and stage10 scope are required"}
var AcceptanceMissing = &AcceptanceError{404, "", "Vendor acceptance or its released PO not found"}
var AcceptanceStale = &AcceptanceError{409, "", "Vendor acceptance or released PO changed; retain your draft, reload and review"}
var acceptanceUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func AcceptanceID(s string) error {
	if !acceptanceUuid.MatchString(s) {
		return AcceptanceInvalid("id", "Use a valid identifier")
	}
	return nil
}
func AcceptanceText(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || (required && s == "") {
		return s, AcceptanceInvalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func AcceptanceToday(t time.Time) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}
func AcceptanceDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type AcceptanceAccess struct{ Read, Write, Scoped, Work bool }

// Order is a minimal immutable source projection; prices and supplier identity
// come exclusively from the saved released PO, never an acceptance command.
type AcceptanceOrder struct {
	ID           string              `json:"id"`
	ProjectID    string              `json:"project_id"`
	PackageID    string              `json:"package_id"`
	Version      int32               `json:"version"`
	Number       string              `json:"number"`
	Date         string              `json:"date"`
	SupplierID   string              `json:"supplier_id"`
	SupplierName string              `json:"supplier_name"`
	Description  string              `json:"description"`
	Quantity     string              `json:"quantity"`
	UOM          string              `json:"uom"`
	Currency     string              `json:"currency"`
	Total        string              `json:"total"`
	PaymentTerms string              `json:"payment_terms"`
	DeliveryDays int                 `json:"delivery_days"`
	Warranty     string              `json:"warranty"`
	ReleasedAt   AcceptanceTimestamp `json:"released_at"`
	ReleasedBy   string              `json:"released_by_id"`
}
type AcceptanceSource struct {
	ProjectID string           `json:"project_id"`
	PackageID string           `json:"package_id"`
	Order     *AcceptanceOrder `json:"order"`
	Available bool             `json:"available"`
	Blockers  []string         `json:"blockers"`
}
type AcceptanceResponse struct {
	Status            string `json:"status"`
	AcceptedDate      string `json:"accepted_date"`
	SignatoryName     string `json:"signatory_name"`
	DocumentReference string `json:"document_reference"`
	Remarks           string `json:"remarks"`
}
type AcceptanceRecord struct {
	ID        string              `json:"id"`
	ProjectID string              `json:"project_id"`
	PackageID string              `json:"package_id"`
	Version   int32               `json:"version"`
	Order     AcceptanceOrder     `json:"order_snapshot"`
	Response  AcceptanceResponse  `json:"response"`
	Active    bool                `json:"is_active"`
	CreatedAt AcceptanceTimestamp `json:"created_at"`
	UpdatedAt AcceptanceTimestamp `json:"updated_at"`
	CreatedBy string              `json:"created_by_id"`
	UpdatedBy string              `json:"updated_by_id"`
}
type AcceptanceCommand struct {
	Version  int32              `json:"version"`
	Action   string             `json:"action"`
	Reason   string             `json:"reason"`
	Response AcceptanceResponse `json:"response"`
}
type AcceptanceEvent struct {
	ID           string              `json:"id"`
	AcceptanceID string              `json:"acceptance_id"`
	Action       string              `json:"action"`
	Reason       string              `json:"reason"`
	Before       *AcceptanceRecord   `json:"before"`
	After        AcceptanceRecord    `json:"after"`
	At           AcceptanceTimestamp `json:"at"`
	ActorID      string              `json:"actor_id"`
}
type AcceptanceWorkspace struct {
	Record   *AcceptanceRecord `json:"record"`
	Source   AcceptanceSource  `json:"source"`
	Actions  []string          `json:"actions"`
	History  []AcceptanceEvent `json:"history"`
	TaskOnly bool              `json:"task_only"`
}
