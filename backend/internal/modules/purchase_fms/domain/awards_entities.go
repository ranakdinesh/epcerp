package domain

// AwardCandidate retains Finance authority privately; the UI projection is explicit.
type AwardCandidate struct {
	BOID    string
	Attempt QuotationApprovalsAttempt
	Lines   []AwardLine
}
type AwardLine struct {
	DemandID     string
	Approval     QuotationApprovalsLine
	Price        QuotationApprovalsCommercialPrice
	DeliveryDays int
	PaymentTerms string
	Warranty     string
}
type AwardRow struct {
	LOIReviewComplete bool           `json:"loi_review_complete"`
	ID                string         `json:"id"`
	BOID              string         `json:"bo_id"`
	Group             string         `json:"group"`
	Vendor            string         `json:"vendor"`
	Reference         string         `json:"reference"`
	Currency          string         `json:"currency"`
	Total             string         `json:"total"`
	Items             int            `json:"items"`
	State             string         `json:"state"`
	Packages          []AwardPackage `json:"packages"`
}
type AwardPackage struct {
	ID           string `json:"id"`
	Description  string `json:"description"`
	Quantity     string `json:"quantity"`
	UOM          string `json:"uom"`
	State        string `json:"state"`
	LOIReference string `json:"loi_reference"`
}
type AwardHandoff struct {
	ID, ProjectID, BOID, AttemptID string
	Packages                       []AwardPackage
	CreatedBy                      string
	CreatedAt                      WorkflowTimestamp
}
type AwardCommand struct {
	AttemptID   string `json:"attempt_id"`
	DoerID      string `json:"doer_id"`
	ResolverID  string `json:"resolver_id"`
	PlannedDate string `json:"planned_date"`
	Reason      string `json:"reason"`
}
type AwardWorkspace struct {
	Rows     []AwardRow       `json:"rows"`
	Members  []WorkflowMember `json:"members"`
	CanWrite bool             `json:"can_write"`
}
