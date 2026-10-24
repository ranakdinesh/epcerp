package domain

import (
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type QuotationApprovalsTimestamp = time.Time
type QuotationApprovalsError = Error

func QuotationApprovalsInvalid(f, m string) error { return &QuotationApprovalsError{422, f, m} }

var QuotationApprovalsUnauthorized = &QuotationApprovalsError{401, "", "Invalid or expired credentials"}
var QuotationApprovalsForbidden = &QuotationApprovalsError{403, "", "Quotation approval permission or project scope denied"}
var QuotationApprovalsMissing = &QuotationApprovalsError{404, "", "Current quotation approval request not found in this project BO"}
var QuotationApprovalsStale = &QuotationApprovalsError{409, "", "Quotation approvals changed; reload"}

type QuotationApprovalsActions struct{ Read, Submit, ReviewRead, Review, Verify, BudgetRead, BudgetWrite bool }

var quotationApprovalsUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var quotationApprovalsKeyPattern = regexp.MustCompile(`^[0-9a-f-]{36}:[1-9][0-9]{0,9}$`)
var quotationApprovalsMoneyPattern = regexp.MustCompile(`^[0-9]{1,14}(?:\.[0-9]{1,2})?$`)

func QuotationApprovalsID(s string) (string, error) {
	if !quotationApprovalsUuid.MatchString(s) {
		return s, QuotationApprovalsInvalid("id", "Use a valid record identifier")
	}
	return s, nil
}
func quotationApprovalsMoney(s string) (*big.Int, error) {
	if !quotationApprovalsMoneyPattern.MatchString(s) {
		return nil, QuotationApprovalsInvalid("amount", "Use an exact monetary amount")
	}
	p := strings.SplitN(s, ".", 2)
	f := "00"
	if len(p) == 2 {
		f = (p[1] + "00")[:2]
	}
	n, ok := new(big.Int).SetString(p[0]+f, 10)
	if !ok {
		return nil, QuotationApprovalsInvalid("amount", "Invalid amount")
	}
	return n, nil
}
func quotationApprovalsDifference(quoted, budget string) (string, error) {
	q, e := quotationApprovalsMoney(quoted)
	if e != nil {
		return "", e
	}
	b, e := quotationApprovalsMoney(budget)
	if e != nil {
		return "", e
	}
	n := new(big.Int).Sub(q, b)
	sign := ""
	if n.Sign() < 0 {
		sign = "-"
		n.Abs(n)
	}
	whole, frac := new(big.Int), new(big.Int)
	whole.QuoRem(n, big.NewInt(100), frac)
	f := frac.String()
	if len(f) < 2 {
		f = "0" + f
	}
	return sign + whole.String() + "." + f, nil
}

type QuotationApprovalsItem struct {
	ID             string `json:"id"`
	SKU            string `json:"sku"`
	Description    string `json:"description"`
	UOM            string `json:"uom"`
	Manufacturer   string `json:"manufacturer"`
	EngineeringKey string `json:"engineering_key"`
}
type QuotationApprovalsCommercialPrice struct {
	DiscountPercent string `json:"discount_percent"`
	TaxPercent      string `json:"tax_percent"`
	Mode            string `json:"pricing_mode"`
	Rate            string `json:"unit_price"`
	Base            string `json:"base"`
	Discount        string `json:"discount"`
	Tax             string `json:"tax"`
	Charges         string `json:"allocated_charges"`
}
type QuotationApprovalsCommercialLine struct {
	AttemptID string                            `json:"attempt_id"`
	LineID    string                            `json:"line_id"`
	ItemID    string                            `json:"item_id"`
	VendorID  string                            `json:"vendor_id"`
	QuoteKey  string                            `json:"quote_key"`
	Quantity  string                            `json:"quantity"`
	Total     string                            `json:"total"`
	Price     QuotationApprovalsCommercialPrice `json:"price"`
}
type QuotationApprovalsQuoteLine struct {
	Coverage, Notes          string
	Months                   *int32
	Price                    QuotationApprovalsCommercialPrice
	LineID, Quantity, Quoted string
	Item                     QuotationApprovalsItem
}
type QuotationApprovalsQuote struct {
	Lowest                                                                  bool
	Key, DocumentID, GroupID, GroupName, VendorID, Vendor, Reference, Total string
	Revision, ScopeRevision                                                 int32
	Terms, ValidUntil                                                       string
	Eligible                                                                bool
	Exclusions                                                              []string
	Lines                                                                   []QuotationApprovalsQuoteLine
}
type QuotationApprovalsBudgetAllocation struct{ BOID, LineID, Amount string }
type QuotationApprovalsBudget struct {
	Revision             int32
	Currency, DecisionID string
	Allocations          []QuotationApprovalsBudgetAllocation
}
type QuotationApprovalsVerification struct {
	State       string `json:"state"`
	Fingerprint string `json:"fingerprint"`
	Message     string `json:"message"`
}
type QuotationApprovalsGroup struct {
	ID, Name      string
	ScopeRevision int32
}
type QuotationApprovalsSource struct {
	ProjectID, BOID, Currency string
	Available                 bool
	Groups                    []QuotationApprovalsGroup
	Quotes                    []QuotationApprovalsQuote
	Budget                    *QuotationApprovalsBudget
	Verifications             map[string]QuotationApprovalsVerification
}
type QuotationApprovalsLine struct {
	Price             *QuotationApprovalsCommercialPrice `json:"price,omitempty"`
	Coverage          string                             `json:"coverage,omitempty"`
	Months            *int32                             `json:"months,omitempty"`
	ExecutiveItemNote string                             `json:"executive_item_note,omitempty"`
	LineID            string                             `json:"line_id"`
	Item              QuotationApprovalsItem             `json:"item"`
	Quantity          string                             `json:"quantity"`
	Budget            *string                            `json:"budget,omitempty"`
	Quoted            string                             `json:"quoted"`
	Difference        *string                            `json:"difference,omitempty"`
	Decision          *string                            `json:"decision"`
	Comment           string                             `json:"comment"`
}
type QuotationApprovalsNomination struct {
	QuoteKey         string `json:"quote_key"`
	BestPrice        bool   `json:"best_price"`
	Preferred        bool   `json:"preferred"`
	BestPriceComment string `json:"best_price_comment"`
	PreferredComment string `json:"preferred_comment"`
}
type QuotationApprovalsProposal struct {
	QuotationApprovalsNomination
	Snapshot QuotationApprovalsAttempt `json:"snapshot"`
}
type QuotationApprovalsAttempt struct {
	BudgetIssue   string                       `json:"budget_issue,omitempty"`
	Terms         string                       `json:"terms,omitempty"`
	ValidUntil    string                       `json:"valid_until,omitempty"`
	ReviewComment string                       `json:"review_comment,omitempty"`
	Proposals     []QuotationApprovalsProposal `json:"proposals,omitempty"`
	ReturnAction  string                       `json:"return_action,omitempty"`

	Currency                string                       `json:"currency"`
	ID                      string                       `json:"id"`
	QuoteKey                string                       `json:"quote_key"`
	DocumentID              string                       `json:"document_id"`
	Revision                int32                        `json:"revision"`
	GroupID                 string                       `json:"group_id"`
	GroupName               string                       `json:"group_name"`
	VendorID                string                       `json:"vendor_id"`
	Vendor                  string                       `json:"vendor"`
	Reference               string                       `json:"reference"`
	Total                   string                       `json:"total"`
	ScopeRevision           int32                        `json:"scope_revision"`
	BudgetRevision          int32                        `json:"budget_revision"`
	VerificationFingerprint string                       `json:"verification_fingerprint"`
	Lines                   []QuotationApprovalsLine     `json:"lines"`
	State                   string                       `json:"state"`
	EffectiveState          string                       `json:"effective_state,omitempty"`
	Message                 string                       `json:"message,omitempty"`
	SubmittedBy             string                       `json:"submitted_by_id"`
	SubmittedAt             QuotationApprovalsTimestamp  `json:"submitted_at"`
	Reason                  string                       `json:"reason"`
	ReviewedBy              *string                      `json:"reviewed_by_id"`
	ReviewedAt              *QuotationApprovalsTimestamp `json:"reviewed_at"`
	DecisionID              *string                      `json:"decision_id"`
}
type QuotationApprovalsRecord struct {
	ProjectID string                      `json:"project_id"`
	BOID      string                      `json:"bo_id"`
	Version   int32                       `json:"version"`
	Attempts  []QuotationApprovalsAttempt `json:"attempts"`
	Active    bool                        `json:"is_active"`
	CreatedAt QuotationApprovalsTimestamp `json:"created_at"`
	UpdatedAt QuotationApprovalsTimestamp `json:"updated_at"`
	CreatedBy *string                     `json:"created_by_id"`
	UpdatedBy *string                     `json:"updated_by_id"`
}
type QuotationApprovalsEvent struct {
	ProjectLabel string                      `json:"-"`
	ID           string                      `json:"id"`
	ProjectID    string                      `json:"project_id"`
	BOID         string                      `json:"bo_id"`
	Action       string                      `json:"action"`
	Reason       string                      `json:"reason"`
	Before       *QuotationApprovalsRecord   `json:"before"`
	After        QuotationApprovalsRecord    `json:"after"`
	ActorID      string                      `json:"created_by_id"`
	CreatedAt    QuotationApprovalsTimestamp `json:"created_at"`
}
type QuotationApprovalsWorkspace struct {
	Drafts          []QuotationShortlistDraft                 `json:"drafts"`
	ProjectName     string                                    `json:"project_name"`
	ProjectNumber   string                                    `json:"project_number"`
	Version         int32                                     `json:"version"`
	Attempts        []QuotationApprovalsAttempt               `json:"attempts"`
	VendorStatuses  map[string]QuotationApprovalsVerification `json:"vendor_statuses"`
	Ready           bool                                      `json:"ready"`
	CanSubmit       bool                                      `json:"can_submit"`
	CanReviewRead   bool                                      `json:"can_review_read"`
	CanReview       bool                                      `json:"can_review"`
	CanReadBudget   bool                                      `json:"can_read_budget"`
	CanUpdateBudget bool                                      `json:"can_update_budget"`
	CanVerify       bool                                      `json:"can_verify"`
	Currency        string                                    `json:"currency"`
	History         []QuotationApprovalsEvent                 `json:"history"`
}
type QuotationApprovalsDecision struct {
	LineID   string `json:"line_id"`
	Decision string `json:"decision"`
	Comment  string `json:"comment"`
}
type QuotationApprovalsCommand struct {
	Proposals        []QuotationApprovalsNomination `json:"proposals,omitempty"`
	SelectedQuoteKey string                         `json:"selected_quote_key,omitempty"`
	ReturnAction     string                         `json:"return_action,omitempty"`

	Version   int32                        `json:"version"`
	Reason    string                       `json:"reason"`
	QuoteKey  string                       `json:"quote_key,omitempty"`
	AttemptID string                       `json:"attempt_id,omitempty"`
	Decisions []QuotationApprovalsDecision `json:"decisions,omitempty"`
}

func (c QuotationApprovalsCommand) Validate(action string) (QuotationApprovalsCommand, error) {
	c.Reason = strings.TrimSpace(c.Reason)
	if c.Version < 0 || c.Reason == "" || utf8.RuneCountInString(c.Reason) > 2000 {
		return c, QuotationApprovalsInvalid("reason", "Provide the saved version and a reason of at most 2000 characters")
	}
	if action == "submit" {
		if len(c.Proposals) > 0 {
			if c.QuoteKey != "" || c.AttemptID != "" || len(c.Decisions) != 0 || c.SelectedQuoteKey != "" || c.ReturnAction != "" {
				return c, QuotationApprovalsInvalid("proposals", "Submit only saved shortlist nominations")
			}
			var e error
			c.Proposals, e = QuotationApprovalsValidateNominations(c.Proposals)
			return c, e
		}
		if c.SelectedQuoteKey != "" || c.ReturnAction != "" {
			return c, QuotationApprovalsInvalid("quote_key", "Review fields cannot be submitted")
		}
		if !quotationApprovalsKeyPattern.MatchString(c.QuoteKey) || len(c.QuoteKey) > 60 || !quotationApprovalsUuid.MatchString(strings.Split(c.QuoteKey, ":")[0]) || c.AttemptID != "" || len(c.Decisions) != 0 {
			return c, QuotationApprovalsInvalid("quote_key", "Choose a saved quotation revision only")
		}
	} else if action == "review" {
		if len(c.Proposals) != 0 || (c.SelectedQuoteKey != "" && !quotationApprovalsKeyPattern.MatchString(c.SelectedQuoteKey)) || (c.ReturnAction != "" && c.ReturnAction != "negotiate" && c.ReturnAction != "change_vendor") {
			return c, QuotationApprovalsInvalid("return_action", "Choose negotiate or change vendor when rejecting")
		}
		if !quotationApprovalsUuid.MatchString(c.AttemptID) || c.QuoteKey != "" || len(c.Decisions) < 1 || len(c.Decisions) > 100 {
			return c, QuotationApprovalsInvalid("decisions", "Review every item in the current submission")
		}
		c.Decisions = append([]QuotationApprovalsDecision{}, c.Decisions...)
		for i := range c.Decisions {
			d := &c.Decisions[i]
			d.Comment = strings.TrimSpace(d.Comment)
			if !quotationApprovalsUuid.MatchString(d.LineID) || (d.Decision != "accept" && d.Decision != "reject") || utf8.RuneCountInString(d.Comment) > 2000 {
				return c, QuotationApprovalsInvalid("decisions", "Each item needs accept/reject; optional notes must be at most 2000 characters")
			}
		}
	} else {
		return c, QuotationApprovalsInvalid("action", "Choose submit or review")
	}
	return c, nil
}
func QuotationApprovalsCopyRecord(r QuotationApprovalsRecord) QuotationApprovalsRecord {
	r.Attempts = QuotationApprovalsCopyAttempts(r.Attempts)
	return r
}
func QuotationApprovalsCopyAttempts(rows []QuotationApprovalsAttempt) []QuotationApprovalsAttempt {
	out := append([]QuotationApprovalsAttempt{}, rows...)
	for i := range out {
		out[i].Lines = append([]QuotationApprovalsLine{}, out[i].Lines...)
		out[i].Proposals = append([]QuotationApprovalsProposal{}, out[i].Proposals...)
		for n := range out[i].Proposals {
			out[i].Proposals[n].Snapshot.Lines = append([]QuotationApprovalsLine{}, out[i].Proposals[n].Snapshot.Lines...)
		}
	}
	return out
}

func QuotationApprovalsSnapshot(q QuotationApprovalsQuote, s QuotationApprovalsSource) (QuotationApprovalsAttempt, error) {
	if !q.Eligible {
		return QuotationApprovalsAttempt{}, QuotationApprovalsInvalid("quote_key", "Quotation is not eligible: "+strings.Join(q.Exclusions, "; "))
	}
	v := s.Verifications[q.VendorID]
	if v.State != "verified" {
		vendor := strings.TrimSpace(q.Vendor)
		if vendor == "" {
			vendor = q.VendorID
		}
		message := strings.TrimSpace(v.Message)
		if message == "" {
			message = "Supplier documents require Finance verification"
		}
		return QuotationApprovalsAttempt{}, QuotationApprovalsInvalid("vendor", "Supplier "+vendor+": "+message)
	}
	if len(q.Lines) == 0 {
		return QuotationApprovalsAttempt{}, QuotationApprovalsInvalid("quote_key", "Quotation has no eligible items")
	}
	a := QuotationApprovalsAttempt{Terms: q.Terms, ValidUntil: q.ValidUntil, Currency: s.Currency, QuoteKey: q.Key, DocumentID: q.DocumentID, Revision: q.Revision, GroupID: q.GroupID, GroupName: q.GroupName, VendorID: q.VendorID, Vendor: q.Vendor, Reference: q.Reference, Total: q.Total, ScopeRevision: q.ScopeRevision, VerificationFingerprint: v.Fingerprint, Lines: []QuotationApprovalsLine{}}
	for _, l := range q.Lines {
		a.Lines = append(a.Lines, QuotationApprovalsLine{Price: &l.Price, Coverage: l.Coverage, Months: l.Months, ExecutiveItemNote: l.Notes, LineID: l.LineID, Item: l.Item, Quantity: l.Quantity, Quoted: l.Quoted})
	}
	// Budget availability belongs to Finance's decision, not procurement's handoff.
	a = QuotationApprovalsFinanceBudget(a, s)
	return a, nil
}
func QuotationApprovalsStaleReason(a QuotationApprovalsAttempt, s QuotationApprovalsSource) string {
	if !s.Available {
		return "Project purchasing is inactive, held or closed"
	}
	found := false
	for _, q := range s.Quotes {
		if q.Key == a.QuoteKey && q.Eligible && q.ScopeRevision == a.ScopeRevision {
			found = true
			break
		}
	}
	if !found {
		return "Quotation expired or indent scope changed; finalise a current eligible quotation"
	}
	v := s.Verifications[a.VendorID]
	if v.State != "verified" || v.Fingerprint != a.VerificationFingerprint {
		return "Vendor verification changed; get verified and resubmit"
	}
	return ""
}
func QuotationApprovalsEvaluate(rows []QuotationApprovalsAttempt, s QuotationApprovalsSource) ([]QuotationApprovalsAttempt, map[string]QuotationApprovalsAttempt, bool) {
	out := QuotationApprovalsCopyAttempts(rows)
	latest := map[string]QuotationApprovalsAttempt{}
	for i := range out {
		a := &out[i]
		a.Message = QuotationApprovalsProposalStaleReason(*a, s)
		if a.Message != "" {
			a.EffectiveState = "stale"
		} else {
			a.EffectiveState = a.State
			a.Message = map[string]string{"pending_accounts": "Awaiting Accounts item approval", "approved": "Accounts approved every item", "rejected": "Accounts rejected this quotation; revise and resubmit"}[a.State]
		}
		latest[a.GroupID] = *a
	}
	ready := len(s.Groups) > 0
	for _, g := range s.Groups {
		a, ok := latest[g.ID]
		if !ok || a.EffectiveState != "approved" {
			ready = false
		}
	}
	return out, latest, ready
}
func QuotationApprovalsDecide(a QuotationApprovalsAttempt, decisions []QuotationApprovalsDecision) (QuotationApprovalsAttempt, error) {
	if len(decisions) != len(a.Lines) {
		return a, QuotationApprovalsInvalid("decisions", "Accept or reject every item exactly once before recording a decision")
	}
	byID := map[string]QuotationApprovalsDecision{}
	for _, d := range decisions {
		if _, ok := byID[d.LineID]; ok {
			return a, QuotationApprovalsInvalid("decisions", "Review each item exactly once")
		}
		byID[d.LineID] = d
	}
	a.Lines = append([]QuotationApprovalsLine{}, a.Lines...)
	a.State = "approved"
	for i := range a.Lines {
		l := &a.Lines[i]
		d, ok := byID[l.LineID]
		if !ok || (d.Decision != "accept" && d.Decision != "reject") || utf8.RuneCountInString(strings.TrimSpace(d.Comment)) > 2000 {
			return a, QuotationApprovalsInvalid("decisions", "Every item requires an accept/reject decision; optional notes must be at most 2000 characters")
		}
		l.Decision = &d.Decision
		l.Comment = strings.TrimSpace(d.Comment)
		if d.Decision == "reject" {
			a.State = "rejected"
		}
	}
	return a, nil
}
func QuotationApprovalsRedact(rows []QuotationApprovalsAttempt) []QuotationApprovalsAttempt {
	rows = QuotationApprovalsCopyAttempts(rows)
	for i := range rows {
		rows[i].BudgetIssue = ""
		for n := range rows[i].Lines {
			rows[i].Lines[n].Budget = nil
			rows[i].Lines[n].Difference = nil
		}
		for n := range rows[i].Proposals {
			rows[i].Proposals[n].Snapshot = QuotationApprovalsRedact([]QuotationApprovalsAttempt{rows[i].Proposals[n].Snapshot})[0]
		}
	}
	return rows
}

type QuotationApprovalsPreviewLine struct {
	LineID     string                 `json:"line_id"`
	Item       QuotationApprovalsItem `json:"item"`
	Quantity   string                 `json:"quantity"`
	Budget     *string                `json:"budget"`
	Quoted     string                 `json:"quoted"`
	Difference *string                `json:"difference"`
}
type QuotationApprovalsPreview struct {
	QuoteKey       string                          `json:"quote_key"`
	Currency       string                          `json:"currency"`
	BudgetRevision *int32                          `json:"budget_revision"`
	Lines          []QuotationApprovalsPreviewLine `json:"lines"`
	Message        string                          `json:"message"`
}

func QuotationApprovalsBudgetPreview(q QuotationApprovalsQuote, s QuotationApprovalsSource) (QuotationApprovalsPreview, error) {
	p := QuotationApprovalsPreview{QuoteKey: q.Key, Currency: s.Currency, Lines: []QuotationApprovalsPreviewLine{}, Message: "No approved item budget in the project currency; ask Finance to approve allocations."}
	alloc := map[string]string{}
	if s.Budget != nil && s.Budget.Currency == s.Currency {
		for _, a := range s.Budget.Allocations {
			if a.BOID == s.BOID {
				alloc[a.LineID] = a.Amount
			}
		}
	}
	for _, l := range q.Lines {
		row := QuotationApprovalsPreviewLine{LineID: l.LineID, Item: l.Item, Quantity: l.Quantity, Quoted: l.Quoted}
		if b, ok := alloc[l.LineID]; ok {
			diff, e := quotationApprovalsDifference(l.Quoted, b)
			if e != nil {
				return p, e
			}
			if !strings.HasPrefix(diff, "-") {
				diff = "+" + diff
			}
			row.Budget = &b
			row.Difference = &diff
		}
		p.Lines = append(p.Lines, row)
	}
	if len(alloc) > 0 {
		r := s.Budget.Revision
		p.BudgetRevision = &r
		p.Message = "Difference = quotation minus approved item budget; + over budget, − below budget."
	}
	return p, nil
}
func QuotationApprovalsInvalidReviewer() error {
	return &QuotationApprovalsError{403, "", "Another Accounts user must review this submission"}
}
