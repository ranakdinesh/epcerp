package application

import (
	"context"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

// One capability service; bound read composition never starts an independent transaction.
type ReferenceService struct {
	repo   masterPorts.ReferenceReader
	unit   masterPorts.ReferenceUnitOfWork
	auth   masterPorts.Authorizer
	values masterPorts.ReferenceValues
}

func NewReferenceService(r masterPorts.ReferenceReader) masterPorts.ReferenceReader {
	return &ReferenceService{repo: r}
}
func NewReferenceAdministration(u masterPorts.ReferenceUnitOfWork, a masterPorts.Authorizer, v masterPorts.ReferenceValues) *ReferenceService {
	return &ReferenceService{unit: u, auth: a, values: v}
}
func (s *ReferenceService) Get(ctx context.Context, k, id string) (*masterDomain.Reference, error) {
	if !masterDomain.ReferenceValidKind(k) {
		return nil, masterDomain.ReferenceUnknown
	}
	return s.repo.Get(ctx, k, id)
}
func (s *ReferenceService) actor(ctx context.Context, t, k string, write bool) (sharedIdentity.Principal, error) {
	p, e := s.auth.Resolve(ctx, t)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, masterDomain.ReferenceUnauthorized
	}
	if !masterDomain.ReferenceValidKind(k) {
		return p, masterDomain.ReferenceUnknown
	}
	if !p.Allows(masterDomain.ReferencePermission(k, write)) {
		return p, masterDomain.ReferenceForbidden
	}
	return p, nil
}
func (s *ReferenceService) List(ctx context.Context, t, k, state string, activeOnly bool) ([]masterDomain.Reference, error) {
	if _, e := s.actor(ctx, t, k, false); e != nil {
		return nil, e
	}
	var records []masterDomain.Reference
	e := s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ReferenceRepository) error {
		var e error
		records, e = r.List(ctx, k, state, activeOnly)
		return e
	})
	if e != nil {
		return nil, e
	}
	return records, nil
}
func (s *ReferenceService) Save(ctx context.Context, t, k, id string, in masterDomain.ReferenceInput) (*masterDomain.Reference, error) {
	actor, e := s.actor(ctx, t, k, true)
	if e != nil {
		return nil, e
	}
	in, e = masterDomain.ReferencePrepare(k, in)
	if e != nil {
		return nil, e
	}
	var saved *masterDomain.Reference
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ReferenceRepository) error {
		if e := r.LockMutations(ctx); e != nil {
			return e
		}
		if k == "cities" {
			parent, e := r.Get(ctx, "states", in.StateID)
			if e != nil {
				return e
			}
			if parent == nil || !parent.Active {
				return masterDomain.ReferenceInvalid("Select an active state or union territory")
			}
		}
		create := id == ""
		var record masterDomain.Reference
		if create {
			record.ID = s.values.ID()
			record.Active = true
			record.CreatedBy = &actor.ID
		} else {
			existing, e := r.Get(ctx, k, id)
			if e != nil {
				return e
			}
			if existing == nil {
				return masterDomain.ReferenceMissing
			}
			record = *existing
		}
		record.Name = in.Name
		record.NormalizedName = masterDomain.ReferenceNormalize(in.Name)
		record.Code = in.Code
		record.StateID = in.StateID
		record.UpdatedBy = &actor.ID
		if e := r.Save(ctx, k, record, create); e != nil {
			return e
		}
		var e error
		saved, e = r.Get(ctx, k, record.ID)
		return e
	})
	if e != nil {
		return nil, e
	}
	return saved, nil
}
func (s *ReferenceService) SetActive(ctx context.Context, t, k, id string, active bool) (*masterDomain.Reference, error) {
	actor, e := s.actor(ctx, t, k, true)
	if e != nil {
		return nil, e
	}
	var saved *masterDomain.Reference
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ReferenceRepository) error {
		if e := r.LockMutations(ctx); e != nil {
			return e
		}
		record, e := r.Get(ctx, k, id)
		if e != nil {
			return e
		}
		if record == nil {
			return masterDomain.ReferenceMissing
		}
		if active && k == "cities" {
			parent, e := r.Get(ctx, "states", record.StateID)
			if e != nil {
				return e
			}
			if parent == nil || !parent.Active {
				return masterDomain.ReferenceInvalid("Activate the city's state first; select an active state or union territory")
			}
		}
		if !active && k == "states" {
			if e := r.RetireCities(ctx, id, actor.ID); e != nil {
				return e
			}
		}
		if e := r.Status(ctx, k, id, actor.ID, active); e != nil {
			return e
		}
		saved, e = r.Get(ctx, k, id)
		return e
	})
	if e != nil {
		return nil, e
	}
	return saved, nil
}
