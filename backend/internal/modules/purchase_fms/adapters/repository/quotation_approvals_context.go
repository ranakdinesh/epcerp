package repository

import (
	"context"
	"errors"
	"fmt"

	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	budgets "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type QuotationApprovalsProviders struct {
	Users        auth.AssignmentDirectory
	Projects     projects.ContextReader
	Purchase     contracts.WorkspaceAccessGate
	Accounts     access.Gate
	Quotes       contracts.QuotationsApprovalReader
	Budgets      budgets.ApprovedReader
	Verification masterContracts.VendorVerificationSourceReader
}
type quotationApprovalsSourceContext struct{ p QuotationApprovalsProviders }

func quotationApprovalsSuppress(e error) (bool, error) {
	var d *access.Error
	if errors.As(e, &d) {
		if d.Status == 403 || d.Status == 422 {
			return false, nil
		}
		return false, &domain.QuotationApprovalsError{Status: d.Status, Field: d.Field, Message: d.Message}
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return false, nil
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return false, domain.QuotationApprovalsMissing
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return false, nil
	}
	return e == nil, e
}
func (c *quotationApprovalsSourceContext) Actions(ctx context.Context, a domain.Principal, p string) (domain.QuotationApprovalsActions, error) {
	purchaseCheck := func(code string, write bool) (bool, error) {
		if !a.Can(code) {
			return false, nil
		}
		v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
		ok, e := quotationApprovalsSuppress(e)
		return ok && (!write || v.CanWrite), e
	}
	accountsCheck := func(code string, write bool) (bool, error) {
		if !a.Can(code) {
			return false, nil
		}
		v, e := c.p.Accounts.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
		ok, e := quotationApprovalsSuppress(e)
		return ok && (!write || v.CanWrite), e
	}
	pv, e := purchaseCheck("projects.group_comparisons.view", false)
	if e != nil {
		return domain.QuotationApprovalsActions{}, e
	}
	pr, e := purchaseCheck("projects.group_comparisons.read", false)
	if e != nil {
		return domain.QuotationApprovalsActions{}, e
	}
	pw, e := purchaseCheck("projects.group_comparisons.write", true)
	if e != nil {
		return domain.QuotationApprovalsActions{}, e
	}
	av, e := accountsCheck("accounts.quotation_approvals.view", false)
	if e != nil {
		return domain.QuotationApprovalsActions{}, e
	}
	ar, e := accountsCheck("accounts.quotation_approvals.read", false)
	if e != nil {
		return domain.QuotationApprovalsActions{}, e
	}
	aw, e := accountsCheck("accounts.quotation_approvals.write", true)
	if e != nil {
		return domain.QuotationApprovalsActions{}, e
	}
	vv, e := accountsCheck("accounts.vendor_verification.view", false)
	if e != nil {
		return domain.QuotationApprovalsActions{}, e
	}
	vr, e := accountsCheck("accounts.vendor_verification.read", false)
	if e != nil {
		return domain.QuotationApprovalsActions{}, e
	}
	br, e := accountsCheck("projects.budgets.read", false)
	if e != nil {
		return domain.QuotationApprovalsActions{}, e
	}
	bw, e := accountsCheck("projects.budgets.write", true)
	if e != nil {
		return domain.QuotationApprovalsActions{}, e
	}
	return domain.QuotationApprovalsActions{BudgetRead: br, BudgetWrite: br && bw, Read: (pv && pr) || (av && ar), Submit: pv && pr && pw, ReviewRead: av && ar, Review: av && ar && aw, Verify: vv && vr}, e
}
func (c *quotationApprovalsSourceContext) Source(ctx context.Context, p, b string) (domain.QuotationApprovalsSource, error) {
	q, e := c.p.Quotes.Source(ctx, p, b)
	if e != nil {
		return domain.QuotationApprovalsSource{}, quotationApprovalsSourceError(e)
	}
	source := domain.QuotationApprovalsSource{ProjectID: p, BOID: b, Currency: q.Currency, Available: q.Available, Groups: []domain.QuotationApprovalsGroup{}, Quotes: []domain.QuotationApprovalsQuote{}, Verifications: map[string]domain.QuotationApprovalsVerification{}}
	budget, e := c.p.Budgets.Approved(ctx, p)
	if e != nil {
		return source, quotationApprovalsSourceError(e)
	}
	if budget != nil {
		source.Budget = &domain.QuotationApprovalsBudget{Revision: budget.Revision, Currency: budget.Currency, DecisionID: budget.DecisionID, Allocations: []domain.QuotationApprovalsBudgetAllocation{}}
		for _, a := range budget.Allocations {
			source.Budget.Allocations = append(source.Budget.Allocations, domain.QuotationApprovalsBudgetAllocation{BOID: a.BOID, LineID: a.LineID, Amount: a.Amount})
		}
	}
	for _, g := range q.Groups {
		source.Groups = append(source.Groups, domain.QuotationApprovalsGroup{ID: g.ID, Name: g.Name, ScopeRevision: g.ScopeRevision})
	}
	for _, r := range q.Quotes {
		quote := domain.QuotationApprovalsQuote{Lowest: r.Suggested, Terms: fmt.Sprintf("Delivery: %d days; %s. Payment: %s", r.DeliveryDays, r.DeliveryTerms, r.PaymentTerms), ValidUntil: r.ValidUntil, Key: fmt.Sprintf("%s:%d", r.DocumentID, r.Revision), DocumentID: r.DocumentID, GroupID: r.GroupID, GroupName: r.GroupName, VendorID: r.VendorID, Vendor: r.Vendor, Reference: r.Reference, Total: r.Total, Revision: r.Revision, ScopeRevision: r.ScopeRevision, Eligible: r.Eligible, Exclusions: append([]string{}, r.Exclusions...), Lines: []domain.QuotationApprovalsQuoteLine{}}
		for _, l := range r.Lines {
			amount := ""
			if l.Landed != nil {
				amount = *l.Landed
			}
			i := l.Source.Snapshot
			value := func(p *string) string {
				if p == nil {
					return ""
				}
				return *p
			}
			mode := l.PricingMode
			if mode == "" {
				mode = "unit_price"
			}
			quote.Lines = append(quote.Lines, domain.QuotationApprovalsQuoteLine{Coverage: l.CoverageType, Months: l.WarrantyMonths, Notes: l.Notes, Price: domain.QuotationApprovalsCommercialPrice{DiscountPercent: l.Discount, TaxPercent: l.Tax, Mode: mode, Rate: value(l.UnitPrice), Base: value(l.Base), Discount: value(l.DiscountAmount), Tax: value(l.TaxAmount), Charges: value(l.AllocatedCharges)}, LineID: l.LineID, Quantity: l.Quantity, Quoted: amount, Item: domain.QuotationApprovalsItem{ID: i.ID, SKU: i.SKU, Description: i.Description, UOM: i.UOM, Manufacturer: i.Manufacturer, EngineeringKey: i.EngineeringKey}})
		}
		source.Quotes = append(source.Quotes, quote)
		if _, ok := source.Verifications[r.VendorID]; !ok {
			v, e := c.p.Verification.Source(ctx, r.VendorID)
			if e != nil {
				return source, quotationApprovalsSourceError(e)
			}
			source.Verifications[r.VendorID] = domain.QuotationApprovalsVerification{State: v.State, Fingerprint: v.Fingerprint, Message: v.Message}
		}
	}
	return source, nil
}

func quotationApprovalsSourceError(e error) error {
	var q *contracts.QuotationsError
	if errors.As(e, &q) {
		return &domain.QuotationApprovalsError{Status: q.Status, Field: q.Field, Message: q.Message}
	}
	var b *budgets.Error
	if errors.As(e, &b) {
		return &domain.QuotationApprovalsError{Status: b.Status, Field: b.Field, Message: b.Message}
	}
	return e
}

func (c *quotationApprovalsSourceContext) ProjectLabel(ctx context.Context, p string) (string, string, error) {
	if c.p.Projects == nil {
		return "", "", domain.QuotationApprovalsMissing
	}
	v, e := c.p.Projects.Get(ctx, p, false)
	if e != nil {
		return "", "", e
	}
	if v == nil {
		return "", "", domain.QuotationApprovalsMissing
	}
	return v.Name, v.Number, nil
}

func (c *quotationApprovalsSourceContext) Reviewers(ctx context.Context, p string) ([]domain.Principal, error) {
	if c.p.Users == nil {
		return nil, domain.QuotationApprovalsInvalid("finance", "Finance assignment directory unavailable")
	}
	users, e := c.p.Users.List(ctx)
	if e != nil {
		return nil, e
	}
	out := []domain.Principal{}
	for _, u := range users {
		a := domain.Principal{ID: u.ID, Active: u.Active, Superuser: u.Superuser, Permissions: u.Permissions}
		if !a.Active || !a.Can("accounts.quotation_approvals.write") {
			continue
		}
		caps, e := c.Actions(ctx, a, p)
		if e != nil {
			return nil, e
		}
		if caps.Review {
			out = append(out, a)
		}
	}
	return out, nil
}

func (c *quotationApprovalsSourceContext) ProjectRead(ctx context.Context, a domain.Principal, p string) (bool, error) {
	caps, e := c.Actions(ctx, a, p)
	if e != nil {
		return false, e
	}
	if caps.Read {
		return true, nil
	}
	if !a.Can("projects.master.read") {
		return false, nil
	}
	_, e = c.p.Accounts.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, "projects.master.read", false)
	return quotationApprovalsSuppress(e)
}
