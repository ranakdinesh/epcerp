package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type SourcingProviders struct {
	Projects  projects.ContextReader
	Purchase  contracts.WorkspaceAccessGate
	Access    access.Gate
	Users     auth.AssignmentDirectory
	Work      contracts.WorkflowSourceReader
	Indents   contracts.IndentsReader
	Items     masterContracts.ItemReader
	Suppliers masterContracts.VendorPurchaseDirectory
	BO        contracts.BODemandReader
	Quotes    contracts.QuotationApprovalsApprovedReader
}
type sourcingSourceContext struct{ p SourcingProviders }

func NewSourcingContext(p SourcingProviders) *sourcingSourceContext { return &sourcingSourceContext{p} }
func sourcingMapped(e error) error {
	var quote *contracts.QuotationApprovalsError
	if errors.As(e, &quote) {
		return &domain.SourcingError{Status: quote.Status, Field: quote.Field, Message: quote.Message}
	}

	var item *masterContracts.Error
	if errors.As(e, &item) {
		status := 500
		switch item.Kind {
		case "unauthenticated":
			status = 401
		case "access_denied":
			status = 403
		case "not_found":
			status = 404
		case "conflict":
			status = 409
		case "invalid_input":
			status = 422
		}
		return &domain.SourcingError{Status: status, Field: item.Field, Message: item.Detail}
	}

	var a *access.Error
	if errors.As(e, &a) {
		return &domain.SourcingError{Status: a.Status, Field: a.Field, Message: a.Message}
	}
	var w *contracts.WorkflowError
	if errors.As(e, &w) {
		return &domain.SourcingError{Status: w.Status, Field: w.Field, Message: w.Message}
	}
	var i *contracts.IndentsError
	if errors.As(e, &i) {
		return &domain.SourcingError{Status: i.Status, Field: i.Field, Message: i.Message}
	}
	var b *contracts.BOError
	if errors.As(e, &b) {
		return &domain.SourcingError{Status: b.Status, Field: b.Field, Message: b.Message}
	}
	var p *contracts.WorkspaceInvalid
	if errors.As(e, &p) {
		return domain.SourcingInvalid(p.Field, p.Message)
	}
	if errors.Is(e, contracts.WorkspaceErrForbidden) {
		return domain.SourcingForbidden
	}
	if errors.Is(e, contracts.WorkspaceErrMissing) {
		return domain.SourcingMissing
	}
	if errors.Is(e, contracts.WorkspaceErrUnauthorized) {
		return domain.SourcingUnauthorized
	}
	return e
}
func sourcingCan(a domain.Principal, code string) bool {
	if !a.Active {
		return false
	}
	if a.Superuser {
		return true
	}
	for _, p := range a.Permissions {
		if p == code {
			return true
		}
	}
	return false
}
func (c *sourcingSourceContext) scoped(ctx context.Context, a domain.Principal, p, code string, write bool) (bool, error) {
	v, e := c.p.Purchase.Check(ctx, contracts.WorkspacePrincipal{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	if e == nil {
		return !write || v.CanWrite, nil
	}
	e = sourcingMapped(e)
	var d *domain.SourcingError
	if !errors.As(e, &d) || d.Status != 403 {
		return false, e
	}
	x, e := c.p.Access.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	e = sourcingMapped(e)
	if errors.As(e, &d) && d.Status == 403 {
		return false, nil
	}
	return e == nil && (!write || x.CanWrite), e
}
func (c *sourcingSourceContext) Access(ctx context.Context, a domain.Principal, p, id string) (out domain.SourcingAccess, e error) {
	out.Work = map[int]bool{}
	parent, e := c.p.Projects.Get(ctx, p, true)
	if e != nil {
		return out, e
	}
	if parent == nil {
		return out, domain.SourcingMissing
	}
	u, e := c.p.Users.Get(ctx, a.ID)
	if e != nil {
		return out, e
	}
	if u == nil || !u.Active {
		return out, domain.SourcingUnauthorized
	}
	a.Superuser = a.Superuser && u.Superuser
	live := func(code string) bool { return sourcingCan(a, code) && u.Allows(code) }
	if !live("projects.purchase_sourcing.read") {
		return out, nil
	}
	out.Scoped, e = c.scoped(ctx, a, p, "projects.purchase_sourcing.read", false)
	if e != nil {
		return out, e
	}
	out.Read = out.Scoped
	for _, n := range []int{3, 4, 5} {
		allowed, err := c.p.Work.CanWork(ctx, contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}, p, id, n)
		if err != nil {
			var d *domain.SourcingError
			err = sourcingMapped(err)
			if !errors.As(err, &d) || d.Status != 422 {
				return out, err
			}
		}
		out.Work[n] = allowed
		taskRead, err := c.p.Work.CanReadTask(ctx, contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}, p, id, n)
		if err != nil {
			return out, sourcingMapped(err)
		}
		out.Read = out.Read || taskRead
	}
	out.Write = out.Read && live("projects.purchase_sourcing.write")
	if out.Write {
		out.Cancel, e = c.p.Work.CanCancel(ctx, contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}, p, id, 4)
		if e != nil {
			return out, sourcingMapped(e)
		}
	}
	if out.Scoped && live("projects.purchase_sourcing.approve") {
		out.Approve, e = c.scoped(ctx, a, p, "projects.purchase_sourcing.approve", true)
		if e != nil {
			var d *domain.SourcingError
			if !errors.As(e, &d) || d.Status != 422 {
				return out, e
			}
			out.Approve = false
			e = nil
		}
	}
	if out.Scoped && out.Write {
		out.Configure, e = c.p.Work.CanConfigure(ctx, contracts.WorkflowActor{ID: a.ID, Active: true, Superuser: a.Superuser, Permissions: a.Permissions}, p)
		e = sourcingMapped(e)
	}
	return out, e
}
func (c *sourcingSourceContext) Source(ctx context.Context, p, id string) (domain.SourcingSource, error) {
	record, current, e := c.p.Work.Inspect(ctx, p, id)
	if e != nil {
		return domain.SourcingSource{}, sourcingMapped(e)
	}
	indent, e := c.p.Indents.Get(ctx, p, record.IndentID)
	if e != nil {
		return domain.SourcingSource{}, sourcingMapped(e)
	}
	if indent == nil {
		return domain.SourcingSource{}, domain.SourcingMissing
	}
	out := domain.SourcingSource{PackageID: id, DemandID: record.DemandID, Quantity: current.Quantity, Currency: indent.Data.Currency, Description: current.Description, Available: current.Available, Eligible: current.Eligible, Blockers: append([]string{}, current.Blockers...)}
	if !current.Available {
		out.Blockers = append(out.Blockers, "Project is inactive, held, closed or purchasing is disabled")
	}
	for _, r := range indent.Data.Released {
		if r.DemandID == record.DemandID {
			out.DemandVersion = r.Source.DemandVersion
		}
	}
	for _, s := range record.Steps {
		if s.Number >= 3 && s.Number <= 5 && s.State == "blocked" {
			out.Blockers = append(out.Blockers, "Sourcing preparation has an unresolved blocker")
		}
	}
	return out, nil
}
func (c *sourcingSourceContext) Approvers(ctx context.Context, p string) ([]domain.SourcingMember, error) {
	rows, e := c.p.Users.List(ctx)
	if e != nil {
		return nil, e
	}
	out := []domain.SourcingMember{}
	for _, u := range rows {
		if !u.Active || !u.Allows("projects.purchase_sourcing.read") || !u.Allows("projects.purchase_sourcing.approve") {
			continue
		}
		a := domain.Principal{ID: u.ID, Active: true, Superuser: u.Superuser, Permissions: u.Permissions}
		read, e := c.scoped(ctx, a, p, "projects.purchase_sourcing.read", false)
		if e != nil {
			return nil, e
		}
		approve, e := c.scoped(ctx, a, p, "projects.purchase_sourcing.approve", false)
		if e != nil {
			return nil, e
		}
		if read && approve {
			out = append(out, domain.SourcingMember{ID: u.ID, Label: u.Name})
		}
	}
	return out, nil
}
func sourcingText(v any) string { s, _ := v.(string); return s }
func sourcingContains(v any, id string) bool {
	switch rows := v.(type) {
	case []string:
		for _, s := range rows {
			if s == id {
				return true
			}
		}
	case []any:
		for _, s := range rows {
			if sourcingText(s) == id {
				return true
			}
		}
	}
	return false
}
func (c *sourcingSourceContext) Suppliers(ctx context.Context, p, id string) ([]domain.SourcingSupplier, error) {
	record, _, e := c.p.Work.Source(ctx, p, id)
	if e != nil {
		return nil, sourcingMapped(e)
	}
	indent, e := c.p.Indents.Get(ctx, p, record.IndentID)
	if e != nil {
		return nil, sourcingMapped(e)
	}
	if indent == nil {
		return nil, domain.SourcingMissing
	}
	itemID := ""
	for _, line := range indent.Data.Released {
		if line.DemandID == record.DemandID {
			itemID = line.Source.Item.ID
		}
	}
	if itemID == "" {
		return nil, domain.SourcingInvalid("source", "Released material item evidence is missing")
	}
	item, e := c.p.Items.Approved(ctx, itemID)
	if e != nil {
		return nil, sourcingMapped(e)
	}
	rows, e := c.p.Suppliers.Candidates(ctx)
	if e != nil {
		return nil, e
	}
	// An exact current Finance approval is a project-specific supplier/item
	// relationship. Do not require users to duplicate it in the master catalogue.
	approved := map[string]bool{}
	if c.p.BO != nil && c.p.Quotes != nil {
		link, err := c.p.BO.Demand(ctx, p, record.DemandID)
		if err != nil {
			return nil, sourcingMapped(err)
		}
		if link != nil && link.ItemID == item.ID {
			var attempts []domain.QuotationApprovalsAttempt
			if groups, ok := c.p.Quotes.(contracts.QuotationGroupApprovedReader); ok {
				attempts, err = groups.ApprovedGroups(ctx, p, link.BOID)
			} else {
				attempts, err = c.p.Quotes.Approved(ctx, p, link.BOID)
			}
			if err != nil {
				return nil, sourcingMapped(err)
			}
			for _, attempt := range attempts {
				for _, line := range attempt.Lines {
					if line.LineID == link.LineID && line.Item.ID == item.ID {
						approved[attempt.VendorID] = true
					}
				}
			}
		}
	}
	out := []domain.SourcingSupplier{}
	for _, v := range rows {
		eligible := v.Active && sourcingText(v.Data["qualification"]) == "approved" && (approved[v.ID] || sourcingContains(v.Data["item_ids"], item.ID) || sourcingContains(v.Data["category_ids"], item.CategoryID))
		contact := sourcingText(v.Data["contact"])
		if strings.TrimSpace(contact) == "" {
			contact = sourcingText(v.Data["phone"])
		}
		if strings.TrimSpace(contact) == "" {
			contact = sourcingText(v.Data["email"])
		}
		out = append(out, domain.SourcingSupplier{ID: v.ID, Name: v.Name, Code: v.Code, Version: v.Version, Eligible: eligible, Registration: sourcingText(v.Data["registration_reference"]), Contact: contact, Location: sourcingText(v.Data["location"])})
	}
	return out, nil
}
func (c *sourcingSourceContext) VerifySelection(ctx context.Context, p, id string, offer domain.SourcingOffer) (*domain.SourcingProof, error) {
	record, _, e := c.p.Work.Source(ctx, p, id)
	if e != nil {
		return nil, sourcingMapped(e)
	}
	link, e := c.p.BO.Demand(ctx, p, record.DemandID)
	if e != nil {
		return nil, sourcingMapped(e)
	}
	if link == nil {
		return nil, domain.SourcingInvalid("financial_approval", "Current source has no completed BO linkage")
	}
	if c.p.Quotes == nil {
		return nil, domain.SourcingInvalid("financial_approval", "Current quotation evidence provider is unavailable")
	}
	var attempts []domain.QuotationApprovalsAttempt
	if groups, ok := c.p.Quotes.(contracts.QuotationGroupApprovedReader); ok {
		attempts, e = groups.ApprovedGroups(ctx, p, link.BOID)
	} else {
		attempts, e = c.p.Quotes.Approved(ctx, p, link.BOID)
	}
	if e != nil {
		return nil, sourcingMapped(e)
	}
	var commercial []domain.QuotationApprovalsCommercialLine
	if groups, ok := c.p.Quotes.(contracts.QuotationGroupApprovedReader); ok {
		commercial, e = groups.CommercialGroups(ctx, p, link.BOID)
	} else {
		commercial, e = c.p.Quotes.Commercial(ctx, p, link.BOID)
	}
	if e != nil {
		return nil, sourcingMapped(e)
	}
	for _, a := range attempts {
		if a.VendorID != offer.SupplierID || a.Reference != offer.QuoteReference || a.QuoteKey != offer.EvidenceReference {
			continue
		}
		for _, line := range a.Lines {
			if line.LineID != link.LineID || line.Item.ID != link.ItemID {
				continue
			}
			snapshot, e := json.Marshal(a)
			if e != nil {
				return nil, e
			}
			var price *domain.SourcingCommercialPrice
			for _, current := range commercial {
				if current.AttemptID == a.ID && current.LineID == link.LineID && current.ItemID == link.ItemID && current.VendorID == a.VendorID && current.QuoteKey == a.QuoteKey {
					v := current.Price
					price = &domain.SourcingCommercialPrice{DiscountPercent: v.DiscountPercent, TaxPercent: v.TaxPercent, Mode: v.Mode, Rate: v.Rate, Base: v.Base, Discount: v.Discount, Tax: v.Tax, Charges: v.Charges}
				}
			}
			if price == nil {
				return nil, domain.SourcingInvalid("financial_approval", "Exact current approved price evidence is missing")
			}
			hash := sha256.Sum256(snapshot)
			return &domain.SourcingProof{Commercial: price, Reference: a.ID, Fingerprint: hex.EncodeToString(hash[:]), VendorID: a.VendorID, Quantity: line.Quantity, Total: line.Quoted, Currency: a.Currency, Effective: true, QuoteReference: a.Reference, EvidenceReference: a.QuoteKey}, nil
		}
	}
	return nil, domain.SourcingInvalid("financial_approval", "Select current Finance-approved evidence for this exact BO item and vendor")
}

// Current approved evidence is limited to the package's exact BO item.
// Budgets, comments, actor identities and fingerprints are never exposed.
func (c *sourcingSourceContext) Evidence(ctx context.Context, p, id string) ([]domain.SourcingApprovedEvidence, error) {
	out := []domain.SourcingApprovedEvidence{}
	record, currentSource, e := c.p.Work.Source(ctx, p, id)
	if e != nil {
		return nil, sourcingMapped(e)
	}
	link, e := c.p.BO.Demand(ctx, p, record.DemandID)
	if e != nil {
		return nil, sourcingMapped(e)
	}
	if link == nil || c.p.Quotes == nil {
		return out, nil
	}
	var attempts []domain.QuotationApprovalsAttempt
	if groups, ok := c.p.Quotes.(contracts.QuotationGroupApprovedReader); ok {
		attempts, e = groups.ApprovedGroups(ctx, p, link.BOID)
	} else {
		attempts, e = c.p.Quotes.Approved(ctx, p, link.BOID)
	}
	if e != nil {
		e = sourcingMapped(e)
		var invalid *domain.SourcingError
		if errors.As(e, &invalid) && invalid.Status == 422 {
			return out, nil
		}
		return nil, e
	}
	var commercial []domain.QuotationApprovalsCommercialLine
	if groups, ok := c.p.Quotes.(contracts.QuotationGroupApprovedReader); ok {
		commercial, e = groups.CommercialGroups(ctx, p, link.BOID)
	} else {
		commercial, e = c.p.Quotes.Commercial(ctx, p, link.BOID)
	}
	if e != nil {
		return nil, sourcingMapped(e)
	}
	for _, a := range attempts {
		for _, line := range a.Lines {
			if line.LineID == link.LineID && line.Item.ID == link.ItemID {
				var price *domain.SourcingCommercialPrice
				for _, current := range commercial {
					if current.AttemptID == a.ID && current.LineID == link.LineID && current.ItemID == link.ItemID {
						v := current.Price
						price = &domain.SourcingCommercialPrice{DiscountPercent: v.DiscountPercent, TaxPercent: v.TaxPercent, Mode: v.Mode, Rate: v.Rate, Base: v.Base, Discount: v.Discount, Tax: v.Tax, Charges: v.Charges}
					}
				}
				if price != nil {
					scoped, err := domain.SourcingScopePrice(*price, currentSource.Quantity, line.Quantity)
					if err != nil {
						return nil, err
					}
					price = &scoped
				}
				scopedTotal := ""
				if price != nil {
					offer := domain.SourcingOffer{PricingMode: price.Mode, Discount: price.Discount, Tax: price.Tax, Freight: price.Charges, Other: "0"}
					if price.Mode == "line_total" {
						offer.LineTotal = price.Base
					} else {
						offer.UnitPrice = price.Rate
					}
					scopedTotal, e = domain.SourcingTotal(offer, currentSource.Quantity)
					if e != nil {
						return nil, e
					}
				}
				out = append(out, domain.SourcingApprovedEvidence{ScopedQuantity: currentSource.Quantity, ScopedTotal: scopedTotal, Price: price, ID: a.ID, VendorID: a.VendorID, QuoteReference: a.Reference, EvidenceReference: a.QuoteKey, Quantity: line.Quantity, Currency: a.Currency, Total: line.Quoted})
			}
		}
	}
	return out, nil
}
