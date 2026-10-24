package domain

import (
	"time"
)

type SourcingTimestamp = time.Time

type SourcingError = Error

func SourcingInvalid(f, m string) error { return &SourcingError{422, f, m} }

var SourcingForbidden = &SourcingError{403, "", "Current sourcing authority or assigned task is required"}
var SourcingUnauthorized = &SourcingError{401, "", "Authentication required"}
var SourcingMissing = &SourcingError{404, "", "Sourcing case or offer not found"}
var SourcingStale = &SourcingError{409, "", "Sourcing or its source changed; reload the saved version"}

type SourcingAccess struct {
	Read, Write, Approve, Configure, BudgetRead, Scoped, Cancel bool
	Work                                                        map[int]bool
}
type SourcingMember struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
type SourcingSource struct {
	PackageID     string   `json:"package_id"`
	DemandID      string   `json:"demand_id"`
	DemandVersion int32    `json:"demand_version"`
	Quantity      string   `json:"quantity"`
	Currency      string   `json:"currency"`
	Description   string   `json:"description"`
	Available     bool     `json:"project_available"`
	Eligible      bool     `json:"eligible"`
	Blockers      []string `json:"blockers"`
}
type SourcingSupplier struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	Version      int32  `json:"version"`
	Eligible     bool   `json:"eligible"`
	Registration string `json:"registration"`
	Contact      string `json:"contact"`
	Location     string `json:"location"`
}
type SourcingOffer struct {
	ID                string           `json:"id"`
	SupplierID        string           `json:"supplier_id"`
	Supplier          SourcingSupplier `json:"supplier_snapshot"`
	DueDate           string           `json:"due_date"`
	Terms             string           `json:"rfq_terms"`
	State             string           `json:"state"`
	SentDate          string           `json:"sent_date"`
	Channel           string           `json:"channel"`
	DispatchReference string           `json:"dispatch_reference"`
	QuoteReference    string           `json:"quote_reference"`
	EvidenceReference string           `json:"evidence_reference"`
	ValidUntil        string           `json:"valid_until"`
	PricingMode       string           `json:"pricing_mode,omitempty"`
	LineTotal         string           `json:"line_total,omitempty"`
	UnitPrice         string           `json:"unit_price"`
	Discount          string           `json:"discount"`
	Tax               string           `json:"tax"`
	Freight           string           `json:"freight"`
	Other             string           `json:"other"`
	DeliveryDays      int              `json:"delivery_days"`
	PaymentTerms      string           `json:"payment_terms"`
	Warranty          string           `json:"warranty"`
	Compliant         bool             `json:"compliant"`
	ComplianceNotes   string           `json:"compliance_notes"`
	NoBidReason       string           `json:"no_bid_reason"`
	Active            bool             `json:"is_active"`
	CreatedAt         time.Time        `json:"created_at"`
	CreatedBy         string           `json:"created_by_id"`
	UpdatedAt         time.Time        `json:"updated_at"`
	UpdatedBy         string           `json:"updated_by_id"`
}
type SourcingComparison struct {
	OfferID string `json:"offer_id"`
	Total   string `json:"total"`
	Rank    *int   `json:"rank"`
	Expired bool   `json:"expired"`
}
type SourcingDecision struct {
	Source             SourcingSource       `json:"source"`
	Offer              SourcingOffer        `json:"offer"`
	Comparison         []SourcingComparison `json:"comparison"`
	Terms              string               `json:"terms"`
	Reason             string               `json:"reason"`
	FinancialReference string               `json:"financial_reference"`
}
type SourcingCase struct {
	AwardReference       string            `json:"award_reference,omitempty"`
	ID                   string            `json:"id"`
	ProjectID            string            `json:"project_id"`
	PackageID            string            `json:"package_id"`
	Source               SourcingSource    `json:"source_snapshot"`
	ApproverID           string            `json:"approver_id"`
	Version              int32             `json:"version"`
	State                string            `json:"state"`
	Offers               []SourcingOffer   `json:"offers"`
	SelectedOfferID      string            `json:"selected_offer_id"`
	SelectionReason      string            `json:"selection_reason"`
	LOITerms             string            `json:"loi_terms"`
	ProposedBy           string            `json:"proposed_by_id"`
	ApprovedBy           string            `json:"approved_by_id"`
	ApprovedAt           *time.Time        `json:"approved_at"`
	Decision             *SourcingDecision `json:"decision_snapshot"`
	LOIReference         string            `json:"loi_reference"`
	LOISentDate          string            `json:"loi_sent_date"`
	LOIChannel           string            `json:"loi_channel"`
	LOIDispatchReference string            `json:"loi_dispatch_reference"`
	ReleasedAt           *time.Time        `json:"released_at"`
	Active               bool              `json:"is_active"`
	CreatedAt            time.Time         `json:"created_at"`
	CreatedBy            string            `json:"created_by_id"`
	UpdatedAt            time.Time         `json:"updated_at"`
	UpdatedBy            string            `json:"updated_by_id"`
}
type SourcingCommand struct {
	Version           int32  `json:"version"`
	Action            string `json:"action"`
	Reason            string `json:"reason"`
	ApproverID        string `json:"approver_id,omitempty"`
	OfferID           string `json:"offer_id,omitempty"`
	SupplierID        string `json:"supplier_id,omitempty"`
	DueDate           string `json:"due_date,omitempty"`
	Terms             string `json:"rfq_terms,omitempty"`
	SentDate          string `json:"sent_date,omitempty"`
	Channel           string `json:"channel,omitempty"`
	DispatchReference string `json:"dispatch_reference,omitempty"`
	QuoteReference    string `json:"quote_reference,omitempty"`
	EvidenceReference string `json:"evidence_reference,omitempty"`
	ValidUntil        string `json:"valid_until,omitempty"`
	PricingMode       string `json:"pricing_mode,omitempty"`
	LineTotal         string `json:"line_total,omitempty"`
	UnitPrice         string `json:"unit_price,omitempty"`
	Discount          string `json:"discount,omitempty"`
	Tax               string `json:"tax,omitempty"`
	Freight           string `json:"freight,omitempty"`
	Other             string `json:"other,omitempty"`
	DeliveryDays      int    `json:"delivery_days,omitempty"`
	PaymentTerms      string `json:"payment_terms,omitempty"`
	Warranty          string `json:"warranty,omitempty"`
	Compliant         *bool  `json:"compliant,omitempty"`
	ComplianceNotes   string `json:"compliance_notes,omitempty"`
	LOITerms          string `json:"loi_terms,omitempty"`
	Approve           *bool  `json:"approve,omitempty"`
}
type SourcingEvent struct {
	ID      string        `json:"id"`
	CaseID  string        `json:"subject_id"`
	Action  string        `json:"action"`
	Reason  string        `json:"reason"`
	Before  *SourcingCase `json:"before"`
	After   SourcingCase  `json:"after"`
	At      time.Time     `json:"created_at"`
	ActorID string        `json:"created_by_id"`
}
type SourcingApprovedEvidence struct {
	ScopedQuantity    string                   `json:"scoped_quantity"`
	ScopedTotal       string                   `json:"scoped_total"`
	Price             *SourcingCommercialPrice `json:"price,omitempty"`
	ID                string                   `json:"id"`
	VendorID          string                   `json:"vendor_id"`
	QuoteReference    string                   `json:"quote_reference"`
	EvidenceReference string                   `json:"evidence_reference"`
	Quantity          string                   `json:"quantity"`
	Currency          string                   `json:"currency"`
	Total             string                   `json:"total"`
}
type SourcingWorkspace struct {
	Evidence   []SourcingApprovedEvidence `json:"approved_evidence"`
	Case       *SourcingCase              `json:"case"`
	Source     SourcingSource             `json:"source"`
	Suppliers  []SourcingSupplier         `json:"suppliers"`
	Approvers  []SourcingMember           `json:"approvers"`
	Actions    []string                   `json:"actions"`
	Comparison []SourcingComparison       `json:"comparison"`
	History    []SourcingEvent            `json:"history"`
	TaskOnly   bool                       `json:"task_only"`
}

func SourcingCopy(c SourcingCase) SourcingCase {
	c.Offers = append([]SourcingOffer{}, c.Offers...)
	c.Source.Blockers = append([]string{}, c.Source.Blockers...)
	if c.Decision != nil {
		d := *c.Decision
		d.Source.Blockers = append([]string{}, d.Source.Blockers...)
		d.Comparison = append([]SourcingComparison{}, d.Comparison...)
		for i := range d.Comparison {
			if d.Comparison[i].Rank != nil {
				rank := *d.Comparison[i].Rank
				d.Comparison[i].Rank = &rank
			}
		}
		c.Decision = &d
	}
	if c.ApprovedAt != nil {
		value := *c.ApprovedAt
		c.ApprovedAt = &value
	}
	if c.ReleasedAt != nil {
		value := *c.ReleasedAt
		c.ReleasedAt = &value
	}
	return c
}
