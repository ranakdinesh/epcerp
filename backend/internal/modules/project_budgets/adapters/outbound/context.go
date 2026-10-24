package outbound

import (
	"context"
	"errors"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
	purchaseContracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
)

type Providers struct {
	Scope    access.Gate
	Projects projects.ContextReader
	Clients  masterContracts.ClientDirectory
	BO       purchaseContracts.BOReader
}
type sourceContext struct{ p Providers }

func bridge(e error) error {
	if errors.Is(e, access.ErrForbidden) {
		return domain.Forbidden
	}
	if errors.Is(e, access.ErrUnauthorized) {
		return domain.Unauthorized
	}
	if errors.Is(e, access.ErrMissing) {
		return domain.Missing
	}
	var d *access.Error
	if errors.As(e, &d) {
		return &domain.Error{Status: d.Status, Field: d.Field, Message: d.Message}
	}
	return e
}
func (c *sourceContext) Scope(ctx context.Context, a domain.Actor, p, code string, write bool) (bool, error) {
	v, e := c.p.Scope.Check(ctx, access.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, p, code, write)
	return v.CanWrite, bridge(e)
}
func (c *sourceContext) Capabilities(ctx context.Context, a domain.Actor, p string) (domain.Capabilities, error) {
	check := func(code string) (bool, error) {
		_, e := c.Scope(ctx, a, p, code, true)
		if e == nil {
			return true, nil
		}
		var d *domain.Error
		if errors.As(e, &d) && (d.Status == 403 || (d.Status == 422 && d.Field == "project")) {
			return false, nil
		}
		return false, e
	}
	write, e := check("projects.budgets.write")
	if e != nil {
		return domain.Capabilities{}, e
	}
	approve, e := check("projects.budgets.approve")
	return domain.Capabilities{CanWrite: write, CanApprove: approve}, e
}
func (c *sourceContext) Source(ctx context.Context, id string) (domain.Scope, error) {
	p, e := c.p.Projects.Get(ctx, id, false)
	if e != nil {
		return domain.Scope{}, e
	}
	if p == nil {
		return domain.Scope{}, domain.Missing
	}
	client, e := c.p.Clients.Get(ctx, p.ClientID)
	if e != nil {
		return domain.Scope{}, e
	}
	project := domain.Project{ID: p.ID, Name: p.Name, Currency: p.Currency, Available: p.Active && client != nil && client.Active && p.Lifecycle != "closed" && p.Lifecycle != "on_hold"}
	if p.ContractValue != "" {
		v := p.ContractValue
		project.ContractValue = &v
	}
	owners, e := c.p.Scope.Owners(ctx, id)
	if e != nil {
		return domain.Scope{}, bridge(e)
	}
	out := domain.Scope{Project: project, Owners: []domain.Owner{}, Items: []domain.Item{}}
	for _, o := range owners {
		out.Owners = append(out.Owners, domain.Owner{ID: o.ID, Name: o.Name})
	}
	records, e := c.p.BO.List(ctx, id)
	if e != nil {
		return domain.Scope{}, e
	}
	for _, record := range records {
		if !record.Active {
			continue
		}
		for _, line := range record.Lines {
			quantity := ""
			if line.Quantity != nil {
				quantity = *line.Quantity
			}
			item := domain.Item{BOID: record.ID, LineID: line.ID, BOVersion: record.Version, BOName: record.Name, Quantity: quantity, SKU: line.Snapshot.SKU, Description: line.Snapshot.Description, UOM: line.Snapshot.UOM, Manufacturer: line.Snapshot.Manufacturer, ItemID: line.Snapshot.ID}
			item.Snapshot = domain.ItemSnapshot{SKU: item.SKU, Description: item.Description, UOM: item.UOM, Manufacturer: item.Manufacturer}
			out.Items = append(out.Items, item)
			if len(out.Items) > 1000 {
				return domain.Scope{}, domain.Invalid("items", "At most 1000 purchase items per project budget")
			}
		}
	}
	return out, nil
}
