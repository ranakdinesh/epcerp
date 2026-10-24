package application

import (
	"context"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type ClientService struct {
	unit   masterPorts.ClientUnitOfWork
	auth   masterPorts.Authorizer
	values masterPorts.ClientValues
}

func NewClientService(u masterPorts.ClientUnitOfWork, a masterPorts.Authorizer, v masterPorts.ClientValues) *ClientService {
	return &ClientService{u, a, v}
}
func (s *ClientService) actor(ctx context.Context, t string, write bool) (sharedIdentity.Principal, error) {
	p, e := s.auth.Resolve(ctx, t)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, masterDomain.ClientUnauthorized
	}
	permission := "settings.clients.read"
	if write {
		permission = "settings.clients.write"
	}
	if !p.Allows(permission) {
		return p, masterDomain.ClientForbidden
	}
	return p, nil
}
func (s *ClientService) Get(ctx context.Context, t, id string) (*masterDomain.Client, error) {
	if _, e := s.actor(ctx, t, false); e != nil {
		return nil, e
	}
	var c *masterDomain.Client
	e := s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ClientRepository, _ masterPorts.ClientDirectory) error {
		var e error
		c, e = r.Get(ctx, id, false)
		if e == nil && c == nil {
			return masterDomain.ClientMissing
		}
		return e
	})
	return c, e
}
func (s *ClientService) List(ctx context.Context, t string, f masterDomain.ClientFilter) (masterDomain.ClientPage, error) {
	if _, e := s.actor(ctx, t, false); e != nil {
		return masterDomain.ClientPage{}, e
	}
	var p masterDomain.ClientPage
	e := s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ClientRepository, _ masterPorts.ClientDirectory) error {
		var e error
		p, e = r.List(ctx, f)
		return e
	})
	return p, e
}
func (s *ClientService) Save(ctx context.Context, t, id string, in masterDomain.ClientInput) (*masterDomain.Client, error) {
	actor, e := s.actor(ctx, t, true)
	if e != nil {
		return nil, e
	}
	var saved *masterDomain.Client
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ClientRepository, d masterPorts.ClientDirectory) error {
		// Resolve selections in the SAME transaction via consumer-owned directory port.
		for _, selection := range []struct {
			kind, id, field string
			target          *string
		}{{"client-types", in.ClientTypeID, "client type", &in.ClientType}, {"industries", in.IndustryID, "industry", &in.Industry}, {"states", in.StateID, "state", &in.State}, {"cities", in.CityID, "city", &in.City}} {
			v, e := d.Get(ctx, selection.kind, selection.id)
			if e != nil {
				return e
			}
			if v == nil || !v.Active {
				return masterDomain.ClientInvalid("The selected " + selection.field + " is not active")
			}
			if selection.kind == "cities" && v.StateID != in.StateID {
				return masterDomain.ClientInvalid("The selected city does not belong to the selected state")
			}
			*selection.target = v.Name
		}
		if e := masterDomain.ClientValidateState(in.Gstin, in.State); e != nil {
			return e
		}
		create := id == ""
		var c masterDomain.Client
		if create {
			c.ID = s.values.ID()
			c.Code = masterDomain.ClientCode(s.values.ID())
			c.Active = true
			c.CreatedBy = &actor.ID
		} else {
			existing, e := r.Get(ctx, id, true)
			if e != nil {
				return e
			}
			if existing == nil {
				return masterDomain.ClientMissing
			}
			c = *existing
		}
		if e := masterDomain.ClientPrepare(in, &c); e != nil {
			return e
		}
		c.UpdatedBy = &actor.ID
		if !create {
			known := map[string]bool{}
			for _, v := range c.Contacts {
				known[v.ID] = true
			}
			for _, v := range in.Contacts {
				if v.ID != "" && !known[v.ID] {
					return masterDomain.ClientInvalid("A contact does not belong to this client")
				}
			}
		}
		contacts := in.Contacts
		for i := range contacts {
			if create || contacts[i].ID == "" {
				contacts[i].ID = s.values.ID()
			}
			contacts[i].Active = true
		}
		if e := r.Save(ctx, c, create); e != nil {
			return e
		}
		if e := r.Contacts(ctx, c.ID, contacts, actor.ID, false); e != nil {
			return e
		}
		var e error
		saved, e = r.Get(ctx, c.ID, false)
		return e
	})
	if e != nil {
		return nil, e
	}
	return saved, nil
}
func (s *ClientService) SetActive(ctx context.Context, t, id string, active bool) (*masterDomain.Client, error) {
	actor, e := s.actor(ctx, t, true)
	if e != nil {
		return nil, e
	}
	var saved *masterDomain.Client
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ClientRepository, _ masterPorts.ClientDirectory) error {
		c, e := r.Get(ctx, id, true)
		if e != nil {
			return e
		}
		if c == nil {
			return masterDomain.ClientMissing
		}
		c.Active = active
		c.UpdatedBy = &actor.ID
		if e = r.Save(ctx, *c, false); e != nil {
			return e
		}
		if !active {
			if e = r.Contacts(ctx, id, nil, actor.ID, true); e != nil {
				return e
			}
		}
		saved, e = r.Get(ctx, id, false)
		return e
	})
	if e != nil {
		return nil, e
	}
	return saved, nil
}
