package domain

import (
	"strings"
	"time"
)

const LOITemplateVersion = 4

type LOICommand struct {
	Reference string `json:"reference"`
	Date      string `json:"date"`
	Format    string `json:"format"`
	Notes     string `json:"notes"`
}

func (q LOICommand) Validate(today string) (LOICommand, error) {
	var e error
	q.Reference, e = WorkflowText("reference", q.Reference, 120, true)
	if e != nil {
		return q, e
	}
	if !WorkflowDate(q.Date) || q.Date > today {
		return q, WorkflowInvalid("date", "Use a valid LOI date that is not in the future")
	}
	switch q.Format {
	case "general", "cable", "panel", "pole":
	default:
		return q, WorkflowInvalid("format", "Choose an LOI format")
	}
	q.Notes = strings.TrimSpace(q.Notes)
	q.Notes, e = WorkflowText("notes", q.Notes, 12000, false)
	return q, e
}

type LOIItem struct {
	Description, SKU, Make, UOM, Quantity string
	Price                                 QuotationApprovalsCommercialPrice
	Total, Payment, Warranty, Technical   string
	DeliveryDays                          int
}
type LOIContent struct {
	LOICommand
	TemplateVersion                                                                  int
	Project, ProjectNumber, Client, Group, Vendor, VendorCode                        string
	Address, ContactPerson, Phone, Email, GST, Quote, Currency, Total, ApprovedTerms string
	SourceHash                                                                       string
	Items                                                                            []LOIItem
}
type LOIDocument struct {
	Review      *LOIReview `json:"review,omitempty"`
	ID          string     `json:"id"`
	Reference   string     `json:"reference"`
	Date        string     `json:"date"`
	Format      string     `json:"format"`
	Notes       string     `json:"notes"`
	Version     int32      `json:"version"`
	CreatedAt   time.Time  `json:"created_at"`
	ProjectID   string     `json:"-"`
	HandoffID   string     `json:"-"`
	AttemptID   string     `json:"-"`
	RequestHash string     `json:"-"`
	SHA256      string     `json:"-"`
	Bucket      string     `json:"-"`
	ObjectKey   string     `json:"-"`
	CreatedBy   string     `json:"-"`
	Size        int64      `json:"-"`
	Content     LOIContent `json:"-"`
}

type LOIReviewCommand struct {
	DetailsCorrect bool `json:"details_correct"`
	PricesCorrect  bool `json:"prices_correct"`
	FormatCorrect  bool `json:"format_correct"`
}

func (q LOIReviewCommand) Validate() error {
	if !q.DetailsCorrect {
		return WorkflowInvalid("details_correct", "Confirm the items, quantities and technical details")
	}
	if !q.PricesCorrect {
		return WorkflowInvalid("prices_correct", "Confirm the approved prices and commercial terms")
	}
	if !q.FormatCorrect {
		return WorkflowInvalid("format_correct", "Confirm the PDF format is correct")
	}
	return nil
}

type LOIReview struct {
	LOIReviewCommand
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	CreatedBy  string    `json:"created_by_id"`
	Reviewer   string    `json:"reviewer"`
	CreatedAt  time.Time `json:"created_at"`
}
type LOIWorkspace struct {
	DefaultReference string        `json:"default_reference"`
	Today            string        `json:"today"`
	CanGenerate      bool          `json:"can_generate"`
	Documents        []LOIDocument `json:"documents"`
}
