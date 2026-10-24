package application

import (
	"context"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type ItemService struct {
	u masterPorts.ItemUnitOfWork
	a masterPorts.Authorizer
	v masterPorts.ItemValues
}

func NewItemService(u masterPorts.ItemUnitOfWork, a masterPorts.Authorizer, v masterPorts.ItemValues) *ItemService {
	return &ItemService{u, a, v}
}
func (s *ItemService) authorize(ctx context.Context, t, permission string) (sharedIdentity.Principal, error) {
	p, e := s.a.Resolve(ctx, t)
	if e != nil || !p.Active || p.ID == "" {
		return p, masterDomain.ItemUnauthorized
	}
	if !p.Allows(permission) {
		return p, masterDomain.ItemForbidden
	}
	return p, nil
}
func (s *ItemService) List(ctx context.Context, t string) (out []masterDomain.Item, e error) {
	if _, e = s.authorize(ctx, t, "settings.items.read"); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.ItemRepository, _ masterPorts.ItemContext) error {
		out, e = r.List(ctx)
		return e
	})
	return
}
func (s *ItemService) References(ctx context.Context, t string) (out masterDomain.ItemReferences, e error) {
	if _, e = s.authorize(ctx, t, "settings.items.read"); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, _ masterPorts.ItemRepository, c masterPorts.ItemContext) error {
		out, e = c.References(ctx)
		return e
	})
	return
}
func (s *ItemService) Create(ctx context.Context, t, configuration, manufacturer string, spec map[string]string, model string) (out masterDomain.Item, e error) {
	p, e := s.authorize(ctx, t, "settings.items.write")
	if e != nil {
		return out, e
	}
	configuration, e = masterDomain.ItemCanonicalID(configuration)
	if e != nil {
		return out, masterDomain.ItemInvalid("configuration_id", "Choose a valid configuration")
	}
	manufacturer, e = masterDomain.ItemCanonicalID(manufacturer)
	if e != nil {
		return out, masterDomain.ItemInvalid("manufacturer_id", "Choose a valid manufacturer")
	}
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.ItemRepository, c masterPorts.ItemContext) error {
		if err := r.Lock(ctx); err != nil {
			return err
		}
		config, err := c.Configuration(ctx, configuration)
		if err != nil {
			return err
		}
		make, err := c.Manufacturer(ctx, manufacturer)
		if err != nil {
			return err
		}
		d, err := masterDomain.ItemDerive(config, make, spec, s.v.NormalizeModel(model))
		if err != nil {
			return err
		}
		engineering, fingerprint := s.v.Keys(d.Identity)
		existing, err := r.Find(ctx, fingerprint)
		if err != nil {
			return err
		}
		if existing != nil {
			out = *existing
			return nil
		}
		// Existing issued evidence stays immutable. Reuse pre-normalization items
		// whose stored model now matches the Python normalization policy.
		items, err := r.Candidates(ctx, d.Identity)
		if err != nil {
			return err
		}
		for _, item := range items {
			_, key := s.v.Keys(masterDomain.ItemIdentity{Family: item.Snapshot.Configuration.FamilyCode, UOM: item.Snapshot.Configuration.UOM, Specifications: item.Specifications, Manufacturer: item.ManufacturerID, Model: s.v.NormalizeModel(item.Model)})
			if key == fingerprint {
				out = item
				return nil
			}
		}
		n, err := r.Next(ctx, config.Data.CategoryCode)
		if err != nil {
			return err
		}
		sku, err := masterDomain.ItemIssue(d.Prefix, n)
		if err != nil {
			return err
		}
		v := masterDomain.Item{ID: s.v.ID(), ConfigurationID: config.ID, ManufacturerID: make.ID, CategoryID: config.CategoryID, Model: d.Model, Specifications: d.Specifications, Snapshot: masterDomain.ItemSnapshot{Configuration: masterDomain.ItemFrozenConfiguration{ItemDefinition: config.Data, Name: config.Name, Code: config.Code}, Manufacturer: make, Prefix: d.Prefix}, EngineeringKey: engineering, Fingerprint: fingerprint, Description: d.Description, SKU: sku, Serial: n, Status: "approved", Active: true, Version: 1}
		out, err = r.Create(ctx, v, p.ID)
		if err != nil {
			return err
		}
		return r.Event(ctx, masterDomain.ItemEvent{ID: s.v.ID(), SubjectID: out.ID, Action: "created", Reason: "Created catalogue item", After: out, Actor: p.ID})
	})
	return
}
func (s *ItemService) Change(ctx context.Context, t, id, status string, version int32, reason string) (out masterDomain.Item, e error) {
	p, e := s.authorize(ctx, t, "settings.items.write")
	if e != nil {
		return out, e
	}
	id, e = masterDomain.ItemCanonicalID(id)
	if e != nil {
		return out, e
	}
	reason, e = masterDomain.ItemReason(reason)
	if e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.ItemRepository, c masterPorts.ItemContext) error {
		if err := r.Lock(ctx); err != nil {
			return err
		}
		before, err := r.Get(ctx, id)
		if err != nil {
			return err
		}
		if err = masterDomain.ItemTransition(before, status, version); err != nil {
			return err
		}
		if status == "approved" {
			config, err := c.Configuration(ctx, before.ConfigurationID)
			if err != nil {
				return err
			}
			make, err := c.Manufacturer(ctx, before.ManufacturerID)
			if err != nil {
				return err
			}
			if !masterDomain.ItemEligible(config, make) {
				return masterDomain.ItemInvalid("", "Configuration or manufacturer is inactive or no longer eligible")
			}
		}
		next := before
		next.Status = status
		next.Active = status == "approved"
		out, err = r.Change(ctx, next, p.ID)
		if err != nil {
			return err
		}
		return r.Event(ctx, masterDomain.ItemEvent{ID: s.v.ID(), SubjectID: id, Action: status, Reason: reason, Before: &before, After: out, Actor: p.ID})
	})
	return
}
func (s *ItemService) History(ctx context.Context, t, id string) (out []masterDomain.ItemEvent, e error) {
	if _, e = s.authorize(ctx, t, "settings.items.read"); e != nil {
		return
	}
	id, e = masterDomain.ItemCanonicalID(id)
	if e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.ItemRepository, _ masterPorts.ItemContext) error {
		if _, err := r.Get(ctx, id); err != nil {
			return err
		}
		out, e = r.History(ctx, id)
		return e
	})
	return
}
