package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type QuotationsError = Error

func QuotationsInvalid(f, m string) error { return &QuotationsError{422, f, m} }

var QuotationsUnauthorized = &QuotationsError{401, "", "Invalid or expired credentials"}
var QuotationsForbidden = &QuotationsError{403, "", "Quotation permission or project scope denied"}
var QuotationsMissing = &QuotationsError{404, "", "Quotation or evidence not found in this project BO"}
var QuotationsStale = &QuotationsError{409, "", "Quotations changed; reload before saving"}

type QuotationsTimestamp = time.Time

var quotationsUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func QuotationsID(s string) (string, error) {
	s = strings.ToLower(s)
	if !quotationsUuid.MatchString(s) {
		return s, QuotationsInvalid("id", "Use a valid record identifier")
	}
	return s, nil
}
func QuotationsDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Year() <= 9999 && t.Format("2006-01-02") == s
}
func QuotationsToday(t time.Time) string {
	return t.In(time.FixedZone("Asia/Kolkata", 19800)).Format("2006-01-02")
}

type QuotationsItem struct {
	ID             string `json:"id"`
	SKU            string `json:"sku"`
	Description    string `json:"description"`
	UOM            string `json:"uom"`
	Manufacturer   string `json:"manufacturer"`
	EngineeringKey string `json:"engineering_key"`
}
type QuotationsSourceLine struct {
	ID           string         `json:"id"`
	ItemID       *string        `json:"item_id"`
	Quantity     *string        `json:"quantity"`
	RequiredDate *string        `json:"required_date"`
	Reference    string         `json:"reference"`
	Notes        string         `json:"notes"`
	Snapshot     QuotationsItem `json:"item_snapshot"`
	DemandID     *string        `json:"demand_id"`
}
type QuotationsVendor struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Code    string         `json:"code"`
	Active  bool           `json:"is_active"`
	Data    map[string]any `json:"data"`
	Version int32          `json:"version"`
}
type QuotationsDocumentData struct {
	Terms         *IndentDocumentsTerms  `json:"terms,omitempty"`
	Number        string                 `json:"number"`
	GroupName     string                 `json:"group_name"`
	ScopeRevision int32                  `json:"scope_revision"`
	IsCurrent     bool                   `json:"is_current"`
	SentDate      *string                `json:"sent_date"`
	Vendor        QuotationsVendor       `json:"vendor"`
	Lines         []QuotationsSourceLine `json:"lines"`
}
type QuotationsDocument struct {
	ID              string                 `json:"id"`
	ProjectID       string                 `json:"project_id"`
	BOID            string                 `json:"bo_id"`
	GroupID         string                 `json:"group_id"`
	VendorID        string                 `json:"vendor_id"`
	Version         int32                  `json:"version"`
	VendorAvailable bool                   `json:"vendor_available"`
	Data            QuotationsDocumentData `json:"data"`
}
type QuotationsLineInput struct {
	LineID         string  `json:"line_id"`
	UnitPrice      *string `json:"unit_price"`
	PricingMode    string  `json:"pricing_mode"`
	LineTotal      *string `json:"line_total"`
	CoverageType   string  `json:"coverage_type"`
	WarrantyMonths *int32  `json:"warranty_months"`
	Discount       string  `json:"discount_pct"`
	Tax            string  `json:"tax_pct"`
	Compliant      bool    `json:"compliant"`
	Notes          string  `json:"notes"`
}
type QuotationsQuoteInput struct {
	DocumentID          string                `json:"document_id"`
	AmountMode          string                `json:"amount_mode"`
	TotalAmount         *string               `json:"total_amount"`
	Reference           string                `json:"reference"`
	AttachmentID        *string               `json:"attachment_id"`
	EvidenceReference   string                `json:"evidence_reference"`
	DocumentNotProvided bool                  `json:"document_not_provided"`
	NoDocumentReason    string                `json:"no_document_reason"`
	ReceivedDate        string                `json:"received_date"`
	ValidUntil          string                `json:"valid_until"`
	Currency            string                `json:"currency"`
	Freight             string                `json:"freight"`
	OtherCharges        string                `json:"other_charges"`
	DeliveryDays        int32                 `json:"delivery_days"`
	DeliveryTerms       string                `json:"delivery_terms"`
	PaymentTerms        string                `json:"payment_terms"`
	Lines               []QuotationsLineInput `json:"lines"`
}
type QuotationsCalcLine struct {
	QuotationsLineInput
	UnitPrice        *string              `json:"unit_price"`
	Quantity         string               `json:"quantity"`
	Source           QuotationsSourceLine `json:"source"`
	Base             *string              `json:"base"`
	DiscountAmount   *string              `json:"discount_amount"`
	Net              *string              `json:"net"`
	TaxAmount        *string              `json:"tax_amount"`
	Landed           *string              `json:"landed"`
	AllocatedCharges *string              `json:"allocated_charges"`
	ItemLowest       bool                 `json:"item_lowest"`
}
type QuotationsCalculation struct {
	Lines     []QuotationsCalcLine `json:"lines"`
	Total     string               `json:"total"`
	Complete  bool                 `json:"complete"`
	Compliant bool                 `json:"compliant"`
}
type QuotationsAttachment struct {
	ID          string    `json:"id"`
	DocumentID  string    `json:"document_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	ByteSize    int64     `json:"byte_size"`
	SHA256      string    `json:"sha256"`
	Active      bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by_id"`
	Ciphertext  []byte    `json:"-"`
}
type QuotationsQuote struct {
	QuotationsQuoteInput
	Calculation QuotationsCalculation `json:"calculation"`
	Attachment  *QuotationsAttachment `json:"attachment"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	CreatedBy   string                `json:"created_by_id"`
	UpdatedBy   string                `json:"updated_by_id"`
	Revision    int32                 `json:"revision"`
	RecordedAt  time.Time             `json:"recorded_at"`
	RecordedBy  string                `json:"recorded_by_id"`
	Reason      string                `json:"reason"`
}
type QuotationsRecord struct {
	ProjectID string            `json:"project_id"`
	BOID      string            `json:"bo_id"`
	Version   int32             `json:"version"`
	Quotes    []QuotationsQuote `json:"quotes"`
	Active    bool              `json:"is_active"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	CreatedBy *string           `json:"created_by_id"`
	UpdatedBy *string           `json:"updated_by_id"`
}
type QuotationsCommand struct {
	Version int32                `json:"version"`
	Reason  string               `json:"reason"`
	Quote   QuotationsQuoteInput `json:"quote"`
}
type QuotationsEvent struct {
	ID        string            `json:"id"`
	BOID      string            `json:"bo_id"`
	Action    string            `json:"action"`
	Reason    string            `json:"reason"`
	Before    *QuotationsRecord `json:"before"`
	After     QuotationsRecord  `json:"after"`
	ActorID   string            `json:"created_by_id"`
	CreatedAt time.Time         `json:"created_at"`
}
type QuotationsScope struct {
	Documents []QuotationsDocument
	Currency  string
	Available bool
}
type QuotationsComparison struct {
	DocumentID    string               `json:"document_id"`
	AmountMode    string               `json:"amount_mode"`
	TotalAmount   *string              `json:"total_amount"`
	GroupID       string               `json:"group_id"`
	GroupName     string               `json:"group_name"`
	ScopeRevision int32                `json:"scope_revision"`
	VendorID      string               `json:"vendor_id"`
	Vendor        string               `json:"vendor"`
	Reference     string               `json:"reference"`
	Total         string               `json:"total"`
	Eligible      bool                 `json:"eligible"`
	Exclusions    []string             `json:"exclusions"`
	Expired       bool                 `json:"expired"`
	DeliveryDays  int32                `json:"delivery_days"`
	PaymentTerms  string               `json:"payment_terms"`
	DeliveryTerms string               `json:"delivery_terms"`
	Lines         []QuotationsCalcLine `json:"lines"`
	Rank          *int32               `json:"rank"`
	Suggested     bool                 `json:"suggested"`
	Revision      int32                `json:"revision"`
	ReceivedDate  string               `json:"received_date"`
	ValidUntil    string               `json:"valid_until"`
	RecordedAt    time.Time            `json:"recorded_at"`
	Reason        string               `json:"reason"`
}
type QuotationsWorkspace struct {
	TaxRate             string                 `json:"tax_rate"`
	Record              *QuotationsRecord      `json:"record"`
	Documents           []QuotationsDocument   `json:"documents"`
	Currency            string                 `json:"currency"`
	CanWrite            bool                   `json:"can_write"`
	History             []QuotationsEvent      `json:"history"`
	Comparisons         []QuotationsComparison `json:"comparisons"`
	ComparisonRevisions []QuotationsComparison `json:"comparison_revisions"`
	Revisions           []QuotationsQuote      `json:"revisions"`
	Attachments         []QuotationsAttachment `json:"attachments"`
}

func quotationsText(field, s string, min, max int) (string, error) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	if n < min || n > max {
		return s, QuotationsInvalid(field, "Record "+field+" within the permitted length")
	}
	return s, nil
}
func (c QuotationsCommand) Validate() (QuotationsCommand, error) {
	var e error
	if c.Version < 0 {
		return c, QuotationsInvalid("version", "Use the current quotation version")
	}
	c.Reason, e = quotationsText("reason", c.Reason, 1, 2000)
	if e != nil {
		return c, e
	}
	q := &c.Quote
	q.DocumentID, e = QuotationsID(q.DocumentID)
	if e != nil {
		return c, e
	}
	if q.AttachmentID != nil {
		v, e := QuotationsID(*q.AttachmentID)
		if e != nil {
			return c, e
		}
		q.AttachmentID = &v
	}
	for _, v := range []struct {
		f        string
		s        *string
		min, max int
	}{{"reference", &q.Reference, 1, 128}, {"evidence_reference", &q.EvidenceReference, 0, 500}, {"no_document_reason", &q.NoDocumentReason, 0, 2000}, {"delivery_terms", &q.DeliveryTerms, 1, 2000}, {"payment_terms", &q.PaymentTerms, 1, 2000}} {
		*v.s, e = quotationsText(v.f, *v.s, v.min, v.max)
		if e != nil {
			return c, e
		}
	}
	if !QuotationsDate(q.ReceivedDate) || !QuotationsDate(q.ValidUntil) {
		return c, QuotationsInvalid("received_date", "Use valid quotation receipt and expiry dates")
	}
	if len(q.Currency) != 3 || q.Currency < "AAA" || q.Currency > "ZZZ" {
		return c, QuotationsInvalid("currency", "Use a three-letter uppercase currency")
	}
	for _, r := range q.Currency {
		if r < 'A' || r > 'Z' {
			return c, QuotationsInvalid("currency", "Use a three-letter uppercase currency")
		}
	}
	if q.DeliveryDays < 0 || q.DeliveryDays > 3650 {
		return c, QuotationsInvalid("delivery_days", "Delivery days must be between zero and 3650")
	}
	if q.AmountMode == "" {
		q.AmountMode = "itemized"
	}
	if q.AmountMode != "itemized" && q.AmountMode != "total_only" {
		return c, QuotationsInvalid("amount_mode", "Choose item pricing or one overall quotation total")
	}
	if q.AmountMode == "total_only" {
		if q.TotalAmount == nil {
			return c, QuotationsInvalid("total_amount", "Enter the vendor's overall quotation total")
		}
		amount, amountError := quotationsAmount(*q.TotalAmount, 20, 2)
		if amountError != nil || amount.Sign() <= 0 {
			return c, QuotationsInvalid("total_amount", "Overall quotation total must be a positive amount")
		}
		q.Freight = "0"
		q.OtherCharges = "0"
	} else if q.TotalAmount != nil {
		return c, QuotationsInvalid("total_amount", "Overall total is only allowed for a total-only quotation")
	}
	if len(q.Lines) == 0 || len(q.Lines) > 100 {
		return c, QuotationsInvalid("lines", "Record from one to 100 quotation items")
	}
	for _, raw := range []string{q.Freight, q.OtherCharges} {
		if _, e := quotationsAmount(raw, 14, 2); e != nil {
			return c, e
		}
	}
	for i, l := range q.Lines {
		l.LineID, e = QuotationsID(l.LineID)
		if e != nil {
			return c, e
		}
		if q.AmountMode == "total_only" {
			l.UnitPrice = nil
			l.LineTotal = nil
			l.PricingMode = ""
			l.CoverageType = ""
			l.WarrantyMonths = nil
			l.Discount = "0"
			l.Tax = "0"
			l.Compliant = false
			l.Notes = ""
			q.Lines[i] = l
			continue
		}
		if l.WarrantyMonths != nil && !quotationsWarrantyInput(l.WarrantyMonths) {
			return c, QuotationsInvalid("warranty_months", "Choose 6, 12, 24 or 48 months")
		}
		if l.PricingMode == "" {
			l.PricingMode = "unit_price"
		}
		if l.CoverageType == "" {
			l.CoverageType = "warranty"
		}
		if l.CoverageType != "warranty" && l.CoverageType != "guarantee" && l.CoverageType != "both" {
			return c, QuotationsInvalid("coverage_type", "Choose warranty, guarantee or both")
		}
		if l.PricingMode != "unit_price" && l.PricingMode != "line_total" {
			return c, QuotationsInvalid("pricing_mode", "Choose unit price or line total")
		}
		if l.UnitPrice != nil {
			if _, e := quotationsAmount(*l.UnitPrice, 12, 2); e != nil {
				return c, e
			}
		}
		if l.LineTotal != nil {
			if _, e := quotationsAmount(*l.LineTotal, 18, 2); e != nil {
				return c, e
			}
		}
		for _, raw := range []string{l.Discount, l.Tax} {
			v, e := quotationsAmount(raw, 3, 2)
			if e != nil {
				return c, e
			}
			if v.Cmp(quotationsHundred) > 0 {
				return c, QuotationsInvalid("tax_pct", "Discount and tax percentages must be from zero to 100")
			}
		}
		l.Notes, e = quotationsText("notes", l.Notes, 0, 2000)
		if e != nil {
			return c, e
		}
		q.Lines[i] = l
	}
	return c, nil
}
func QuotationsCopy(r QuotationsRecord) QuotationsRecord {
	r.Quotes = append([]QuotationsQuote{}, r.Quotes...)
	return r
}

type QuotationsBOChoice struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}
