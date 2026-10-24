package application

import (
	"context"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type CategoryService struct {
	unit   masterPorts.CategoryUnitOfWork
	auth   masterPorts.Authorizer
	values masterPorts.CategoryValues
}

func NewCategoryService(u masterPorts.CategoryUnitOfWork, a masterPorts.Authorizer, v masterPorts.CategoryValues) *CategoryService {
	return &CategoryService{u, a, v}
}
func (s *CategoryService) actor(ctx context.Context, t, p string) (sharedIdentity.Principal, error) {
	a, e := s.auth.Resolve(ctx, t)
	if e != nil {
		return a, e
	}
	if !a.Active || a.ID == "" {
		return a, masterDomain.CategoryUnauthorized
	}
	if !a.Allows(p) {
		return a, masterDomain.CategoryForbidden
	}
	return a, nil
}
func (s *CategoryService) List(ctx context.Context, t string) (out masterDomain.CategoryCatalogue, e error) {
	if _, e = s.actor(ctx, t, "settings.categories.read"); e != nil {
		return
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.CategoryRepository) error { out, e = r.List(ctx); return e })
	return
}
func (s *CategoryService) Import(ctx context.Context, t string, rows []masterDomain.CategorySource) (out masterDomain.CategoryResult, e error) {
	a, e := s.actor(ctx, t, "settings.categories.import")
	if e != nil {
		return out, e
	}
	if e = masterDomain.CategoryValidate(rows); e != nil {
		return out, e
	}
	categoryUnits, e := masterDomain.CategoryUnits(rows)
	if e != nil {
		return out, e
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.CategoryRepository) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		existing, e := r.List(ctx)
		if e != nil {
			return e
		}
		cats := map[string]string{}
		units := map[string]string{}
		save := func(kind string, v masterDomain.Category, source *masterDomain.CategorySource, list *[]masterDomain.Category) (masterDomain.Category, error) {
			for _, old := range *list {
				if old.Code == v.Code {
					if old.Fingerprint != v.Fingerprint || old.Name != v.Name || old.Active != v.Active || old.CategoryID != v.CategoryID || old.UnitID != v.UnitID {
						return old, masterDomain.CategoryConflict
					}
					return old, nil
				}
			}
			if e := r.Insert(ctx, kind, masterDomain.CategoryImportRecord{Record: v, Source: source, Actor: a.ID}); e != nil {
				return v, e
			}
			*list = append(*list, v)
			switch kind {
			case "category":
				out.Categories++
			case "subcategory":
				out.Subcategories++
			case "unit":
				out.Units++
			}
			return v, nil
		}

		for _, source := range rows {
			if source.Data.Kind != "type" {
				continue
			}
			d := source.Data
			if units[d.UOM] == "" {
				v, e := save("unit", masterDomain.Category{ID: s.values.ID(), Code: d.UOM, Name: masterDomain.CategoryUnitName(d.UOM), Active: true, Version: 1, Fingerprint: "controlled-unit-v1:" + d.UOM}, nil, &existing.Units)
				if e != nil {
					return e
				}
				units[d.UOM] = v.ID
			}
		}
		for _, source := range rows {
			if source.Data.Kind != "category" {
				continue
			}
			v, e := save("category", masterDomain.Category{ID: s.values.ID(), Code: source.Code, Name: source.Name, Active: source.Active, Version: 1, Fingerprint: s.values.Fingerprint(source), UnitID: units[categoryUnits[source.Code]]}, &source, &existing.Categories)
			if e != nil {
				return e
			}
			cats[v.Code] = v.ID
		}
		for _, source := range rows {
			if source.Data.Kind != "type" {
				continue
			}
			d := source.Data

			config := &masterDomain.CategoryConfiguration{Fields: d.Fields, FamilyCode: d.FamilyCode, Revision: d.Revision, ModelRequired: d.ModelRequired}
			_, e := save("subcategory", masterDomain.Category{ID: s.values.ID(), Code: source.Code, Name: source.Name, Active: source.Active, Version: 1, Fingerprint: s.values.Fingerprint(source), CategoryID: cats[d.CategoryCode], UnitID: units[d.UOM], Configuration: config}, &source, &existing.Subcategories)
			if e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		return masterDomain.CategoryResult{}, e
	}
	return out, nil
}

func (s *CategoryService) Create(ctx context.Context, t, kind string, in masterDomain.CategoryCreate) (saved masterDomain.Category, e error) {
	a, e := s.actor(ctx, t, "settings.categories.write")
	if e != nil {
		return saved, e
	}
	in, e = masterDomain.CategoryCleanCreate(kind, in)
	if e != nil {
		return saved, e
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.CategoryRepository) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		all, e := r.List(ctx)
		if e != nil {
			return e
		}
		list := all.Units
		v := masterDomain.Category{ID: s.values.ID(), Code: in.Code, Name: in.Name, Active: true, Version: 1}
		switch kind {
		case "category":
			list = all.Categories
			for _, u := range all.Units {
				if u.ID == in.UnitID && u.Active {
					v.UnitID = u.ID
				}
			}
			if v.UnitID == "" {
				return masterDomain.CategoryFieldInvalid("unit_id", "Select an active base unit")
			}
		case "subcategory":
			list = all.Subcategories
			for _, c := range all.Categories {
				if c.ID == in.CategoryID && c.Active {
					v.CategoryID = c.ID
					v.UnitID = c.UnitID
				}
			}
			if v.CategoryID == "" {
				return masterDomain.CategoryFieldInvalid("category_id", "Select an active category")
			}
			activeUnit := false
			for _, u := range all.Units {
				if u.ID == v.UnitID && u.Active {
					activeUnit = true
				}
			}
			if !activeUnit {
				return masterDomain.CategoryFieldInvalid("category_id", "Category base unit is inactive")
			}
			v.Configuration = &masterDomain.CategoryConfiguration{Fields: []masterDomain.CategoryField{}, FamilyCode: in.Code, Revision: 1}
		case "unit":
		}
		for _, old := range list {
			if categoryStringsEqualCode(old.Code, in.Code) {
				return masterDomain.CategoryFieldInvalid("code", "Code already exists")
			}
			if masterDomain.CategorySameName(old.Name, in.Name) && (kind != "subcategory" || old.CategoryID == v.CategoryID) {
				return masterDomain.CategoryFieldInvalid("name", "Name already exists")
			}
		}
		if e := r.Insert(ctx, kind, masterDomain.CategoryImportRecord{Record: v, Actor: a.ID, Reason: in.Reason}); e != nil {
			return e
		}
		all, e = r.List(ctx)
		if e != nil {
			return e
		}
		list = all.Units
		if kind == "category" {
			list = all.Categories
		} else if kind == "subcategory" {
			list = all.Subcategories
		}
		for _, row := range list {
			if row.ID == v.ID {
				saved = row
				return nil
			}
		}
		return masterDomain.CategoryInvalid("Created record unavailable")
	})
	if e != nil {
		return masterDomain.Category{}, e
	}
	return saved, nil
}
func categoryStringsEqualCode(a, b string) bool { return masterDomain.CategorySameName(a, b) }

func (s *CategoryService) Edit(ctx context.Context, t, kind, id string, in masterDomain.CategoryEdit) (saved masterDomain.Category, e error) {
	a, e := s.actor(ctx, t, "settings.categories.write")
	if e != nil {
		return saved, e
	}
	if !masterDomain.CategoryValidID(id) {
		return saved, masterDomain.CategoryInvalid("Invalid record identity")
	}
	originalCode := in.Code
	in, e = masterDomain.CategoryCleanEdit(kind, in)
	if e != nil {
		return saved, e
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.CategoryRepository) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		all, e := r.List(ctx)
		if e != nil {
			return e
		}
		list := all.Units
		if kind == "category" {
			list = all.Categories
		} else if kind == "subcategory" {
			list = all.Subcategories
		}
		var old *masterDomain.Category
		for _, v := range list {
			if v.ID == id {
				row := v
				old = &row
				break
			}
		}
		if old == nil {
			return masterDomain.CategoryMissing
		}
		if masterDomain.CategorySameName(originalCode, old.Code) {
			in.Code = old.Code
		}
		if old.Version != in.Version {
			return masterDomain.CategoryStale
		}
		for _, v := range list {
			if v.ID == id {
				continue
			}
			if masterDomain.CategorySameName(v.Code, in.Code) {
				return masterDomain.CategoryFieldInvalid("code", "Code already exists")
			}
			if masterDomain.CategorySameName(v.Name, in.Name) && (kind != "subcategory" || v.CategoryID == old.CategoryID) {
				return masterDomain.CategoryFieldInvalid("name", "Name already exists")
			}
		}
		next := *old
		next.Code = in.Code
		next.Name = in.Name
		if e = r.Update(ctx, kind, masterDomain.CategoryCorrection{Before: *old, After: next, Actor: a.ID, Reason: in.Reason}); e != nil {
			return e
		}
		all, e = r.List(ctx)
		if e != nil {
			return e
		}
		list = all.Units
		if kind == "category" {
			list = all.Categories
		} else if kind == "subcategory" {
			list = all.Subcategories
		}
		for _, v := range list {
			if v.ID == id {
				saved = v
				return nil
			}
		}
		return masterDomain.CategoryMissing
	})
	if e != nil {
		return masterDomain.Category{}, e
	}
	return saved, nil
}
