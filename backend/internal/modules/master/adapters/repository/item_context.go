package repository

import (
	"context"
	"errors"

	pgx "github.com/jackc/pgx/v5"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type itemItemContext struct {
	categories masterContracts.CategoryItemReader
	templates  masterContracts.TemplateItemReader
	makes      masterContracts.ManufacturerItemReader
}

func itemConfiguration(t masterContracts.Template, c masterContracts.CategoryItemContext) masterDomain.ItemConfiguration {
	family := c.FamilyCode
	if family == "" {
		family = c.Code
	}
	fields := []masterDomain.ItemField{}
	for _, f := range t.Fields {
		if !f.Active {
			continue
		}
		options := []masterDomain.ItemOption{}
		for _, o := range f.Options {
			if o.Active {
				options = append(options, masterDomain.ItemOption{Code: o.Code, Label: o.Label})
			}
		}
		if f.AllowNA {
			options = append(options, masterDomain.ItemOption{Code: "__NA__", Label: "N/A"})
		}
		fields = append(fields, masterDomain.ItemField{Code: f.Code, Label: f.Name, Required: f.Mandatory, Options: options})
	}
	return masterDomain.ItemConfiguration{ID: t.ID, Code: c.Code, Name: c.Name, Active: t.Active && c.Active, Version: t.Revision, CategoryID: c.CategoryID, SubcategoryID: c.ID, Data: masterDomain.ItemDefinition{Kind: "type", CategoryCode: c.CategoryCode, FamilyCode: family, UOM: c.UnitCode, Revision: t.Revision, ModelRequired: t.ModelRequired, Fields: fields}}
}
func itemMakeView(v masterContracts.ManufacturerMake) masterDomain.ItemMake {
	return masterDomain.ItemMake{ID: v.ID, Code: v.Code, Name: v.Name, Active: v.Active, Data: masterDomain.ItemMakeData{Eligible: v.Eligible}}
}
func (c *itemItemContext) Configuration(ctx context.Context, id string) (masterDomain.ItemConfiguration, error) {
	t, e := c.templates.GetRevision(ctx, id)
	if errors.Is(e, pgx.ErrNoRows) {
		return masterDomain.ItemConfiguration{}, masterDomain.ItemInvalid("configuration_id", "Configuration does not exist")
	}
	if e != nil {
		return masterDomain.ItemConfiguration{}, e
	}
	v, e := c.categories.GetItemContext(ctx, t.SubcategoryID)
	if errors.Is(e, pgx.ErrNoRows) {
		return masterDomain.ItemConfiguration{}, masterDomain.ItemInvalid("configuration_id", "Configuration parent is missing")
	}
	if e != nil {
		return masterDomain.ItemConfiguration{}, e
	}
	return itemConfiguration(t, v), nil
}
func (c *itemItemContext) Manufacturer(ctx context.Context, id string) (masterDomain.ItemMake, error) {
	v, e := c.makes.GetMake(ctx, id)
	if e != nil {
		return masterDomain.ItemMake{}, e
	}
	if v.ID == "" {
		return masterDomain.ItemMake{}, masterDomain.ItemInvalid("manufacturer_id", "Manufacturer does not exist")
	}
	return itemMakeView(v), nil
}
func (c *itemItemContext) References(ctx context.Context) (masterDomain.ItemReferences, error) {
	out := masterDomain.ItemReferences{Catalogue: []masterDomain.ItemConfiguration{}, Manufacturers: []masterDomain.ItemMake{}}
	contexts, e := c.categories.ItemContexts(ctx)
	if e != nil {
		return out, e
	}
	templates, e := c.templates.ListLatest(ctx)
	if e != nil {
		return out, e
	}
	byID := map[string]masterContracts.CategoryItemContext{}
	categorySeen := map[string]int{}
	for _, v := range contexts {
		byID[v.ID] = v
		if _, seen := categorySeen[v.CategoryID]; !seen {
			out.Catalogue = append(out.Catalogue, masterDomain.ItemConfiguration{ID: v.CategoryID, Code: v.CategoryCode, Name: v.CategoryName, Active: v.Active, Data: masterDomain.ItemDefinition{Kind: "category", UOM: v.UnitCode, Fields: []masterDomain.ItemField{}}})
			categorySeen[v.CategoryID] = len(out.Catalogue) - 1
		} else if v.Active {
			out.Catalogue[categorySeen[v.CategoryID]].Active = true
		}
	}
	for _, t := range templates {
		if v, ok := byID[t.SubcategoryID]; ok {
			out.Catalogue = append(out.Catalogue, itemConfiguration(t, v))
		}
	}
	ms, e := c.makes.ListMakes(ctx)
	if e != nil {
		return out, e
	}
	for _, m := range ms {
		out.Manufacturers = append(out.Manufacturers, itemMakeView(m))
	}
	return out, nil
}
