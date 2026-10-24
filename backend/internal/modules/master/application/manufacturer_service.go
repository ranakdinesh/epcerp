package application

import (
	"context"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type ManufacturerService struct {
	unit   masterPorts.ManufacturerUnitOfWork
	auth   masterPorts.Authorizer
	values masterPorts.ManufacturerValues
}

func NewManufacturerService(u masterPorts.ManufacturerUnitOfWork, a masterPorts.Authorizer, v masterPorts.ManufacturerValues) *ManufacturerService {
	return &ManufacturerService{u, a, v}
}
func (s *ManufacturerService) actor(ctx context.Context, t string, write bool) (sharedIdentity.Principal, error) {
	p, e := s.auth.Resolve(ctx, t)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, masterDomain.ManufacturerUnauthorized
	}
	permission := "settings.manufacturers.read"
	if write {
		permission = "settings.manufacturers.write"
	}
	if !p.Allows(permission) {
		return p, masterDomain.ManufacturerForbidden
	}
	return p, nil
}
func (s *ManufacturerService) List(ctx context.Context, t string) ([]masterDomain.Manufacturer, error) {
	if _, e := s.actor(ctx, t, false); e != nil {
		return nil, e
	}
	var records []masterDomain.Manufacturer
	e := s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ManufacturerRepository, _ masterPorts.ManufacturerEligibility) error {
		var e error
		records, e = r.List(ctx)
		return e
	})
	if e != nil {
		return nil, e
	}
	return records, nil
}
func (s *ManufacturerService) Create(ctx context.Context, t, code, name string, data masterDomain.ManufacturerData, reason string) (*masterDomain.Manufacturer, error) {
	actor, e := s.actor(ctx, t, true)
	if e != nil {
		return nil, e
	}
	var saved masterDomain.Manufacturer
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ManufacturerRepository, eligibility masterPorts.ManufacturerEligibility) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		clean, e := masterDomain.ManufacturerValidateCode(code)
		if e != nil {
			return e
		}
		records, e := r.List(ctx)
		if e != nil {
			return e
		}
		for _, v := range records {
			if v.Code == clean || masterDomain.ManufacturerNameKey(v.Name) == masterDomain.ManufacturerNameKey(name) {
				return masterDomain.ManufacturerInvalid("Code or name already exists")
			}
		}
		if e := eligibility.Validate(ctx, data); e != nil {
			return e
		}
		record := masterDomain.Manufacturer{ID: s.values.ID(), Code: clean, Name: name, Data: data.Snapshot(), Active: true, Version: 1, CreatedBy: &actor.ID, UpdatedBy: &actor.ID}
		saved, e = r.Create(ctx, record)
		if e != nil {
			return e
		}
		return r.Event(ctx, masterDomain.ManufacturerEvent{ID: s.values.ID(), SubjectID: saved.ID, Action: "create", Reason: reason, Before: map[string]any{}, After: map[string]any{"code": clean, "name": name, "data": data.Snapshot()}, Actor: &actor.ID})
	})
	if e != nil {
		return nil, e
	}
	return &saved, nil
}
func (s *ManufacturerService) Profile(ctx context.Context, t, id string, version int32, name string, data masterDomain.ManufacturerData, reason string) (*masterDomain.Manufacturer, error) {
	actor, e := s.actor(ctx, t, true)
	if e != nil {
		return nil, e
	}
	var saved masterDomain.Manufacturer
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ManufacturerRepository, eligibility masterPorts.ManufacturerEligibility) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		records, e := r.List(ctx)
		if e != nil {
			return e
		}
		for _, v := range records {
			if v.ID != id && masterDomain.ManufacturerNameKey(v.Name) == masterDomain.ManufacturerNameKey(name) {
				return masterDomain.ManufacturerInvalid("Name already exists")
			}
		}
		if e := eligibility.Validate(ctx, data); e != nil {
			return e
		}
		old, e := r.Get(ctx, id, true)
		if e != nil {
			return e
		}
		if old == nil {
			return masterDomain.ManufacturerMissing
		}
		if old.Version != version || version == 2147483647 {
			return masterDomain.ManufacturerStale
		}
		before := map[string]any{"name": old.Name, "data": old.Data, "version": old.Version}
		old.Name = name
		old.Data = data.Snapshot()
		old.Version++
		old.UpdatedBy = &actor.ID
		saved, e = r.Update(ctx, *old, true)
		if e != nil {
			return e
		}
		return r.Event(ctx, masterDomain.ManufacturerEvent{ID: s.values.ID(), SubjectID: id, Action: "update", Reason: reason, Before: before, After: map[string]any{"name": name, "data": data.Snapshot(), "version": saved.Version}, Actor: &actor.ID})
	})
	if e != nil {
		return nil, e
	}
	return &saved, nil
}
func (s *ManufacturerService) SetActive(ctx context.Context, t, id string, version int32, active bool, reason string) (*masterDomain.Manufacturer, error) {
	actor, e := s.actor(ctx, t, true)
	if e != nil {
		return nil, e
	}
	var saved masterDomain.Manufacturer
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ManufacturerRepository, _ masterPorts.ManufacturerEligibility) error {
		old, e := r.Get(ctx, id, true)
		if e != nil {
			return e
		}
		if old == nil {
			return masterDomain.ManufacturerMissing
		}
		if old.Version != version || version == 2147483647 {
			return masterDomain.ManufacturerStale
		}
		before := map[string]any{"is_active": old.Active, "version": old.Version}
		old.Active = active
		old.Version++
		old.UpdatedBy = &actor.ID
		saved, e = r.Update(ctx, *old, false)
		if e != nil {
			return e
		}
		return r.Event(ctx, masterDomain.ManufacturerEvent{ID: s.values.ID(), SubjectID: id, Action: "lifecycle", Reason: reason, Before: before, After: map[string]any{"is_active": active, "version": saved.Version}, Actor: &actor.ID})
	})
	if e != nil {
		return nil, e
	}
	return &saved, nil
}
func (s *ManufacturerService) History(ctx context.Context, t, id string) ([]masterDomain.ManufacturerEvent, error) {
	if _, e := s.actor(ctx, t, false); e != nil {
		return nil, e
	}
	var events []masterDomain.ManufacturerEvent
	e := s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ManufacturerRepository, _ masterPorts.ManufacturerEligibility) error {
		record, e := r.Get(ctx, id, false)
		if e != nil {
			return e
		}
		if record == nil {
			return masterDomain.ManufacturerMissing
		}
		events, e = r.History(ctx, id)
		return e
	})
	if e != nil {
		return nil, e
	}
	return events, nil
}

func (s *ManufacturerService) Options(ctx context.Context, token, search string, codes []string, skip int32, kind, category string) (out masterDomain.ManufacturerSelection, e error) {
	if _, e = s.actor(ctx, token, false); e != nil {
		return
	}
	if len(search) > 128 || len(codes) > 200 || skip < 0 || skip > 1000000 || (kind != "" && kind != "category" && kind != "subcategory") || len(category) > 32 || (category != "" && kind != "subcategory") {
		return out, masterDomain.ManufacturerInvalid("Invalid eligibility query")
	}
	e = s.unit.Within(ctx, func(ctx context.Context, _ masterPorts.ManufacturerRepository, eligible masterPorts.ManufacturerEligibility) error {
		out, e = eligible.Options(ctx, search, codes, skip, kind, category)
		return e
	})
	return
}
func (s *ManufacturerService) Import(ctx context.Context, token string, in masterDomain.ManufacturerLegacy) (out *masterDomain.Manufacturer, e error) {
	p, e := s.auth.Resolve(ctx, token)
	if e != nil {
		return nil, e
	}
	if !p.Active || p.ID == "" {
		return nil, masterDomain.ManufacturerUnauthorized
	}
	if !p.Allows("settings.manufacturers.import") {
		return nil, masterDomain.ManufacturerForbidden
	}
	if e = masterDomain.ManufacturerValidateImport(in); e != nil {
		return nil, e
	}
	e = s.unit.Within(ctx, func(ctx context.Context, r masterPorts.ManufacturerRepository, eligible masterPorts.ManufacturerEligibility) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		existing, fp, e := r.FindImport(ctx, in.Source.ID)
		if e != nil {
			return e
		}
		fingerprint := s.values.Fingerprint(in)
		if existing != nil {
			if fp != fingerprint {
				return masterDomain.ManufacturerConflict
			}
			out = existing
			return nil
		}
		records, e := r.List(ctx)
		if e != nil {
			return e
		}
		for _, v := range records {
			if v.Code == in.Code || masterDomain.ManufacturerNameKey(v.Name) == masterDomain.ManufacturerNameKey(in.Name) {
				return masterDomain.ManufacturerConflict
			}
		}
		if e = eligible.Validate(ctx, in.Data); e != nil {
			return e
		}
		saved, e := r.Create(ctx, masterDomain.Manufacturer{ID: s.values.ID(), Code: in.Code, Name: in.Name, Data: in.Data.Snapshot(), Active: *in.Active, Version: 1, CreatedBy: &p.ID, UpdatedBy: &p.ID})
		if e != nil {
			return e
		}
		if e = r.RecordImport(ctx, saved.ID, in, fingerprint, p.ID); e != nil {
			return e
		}
		if e = r.Event(ctx, masterDomain.ManufacturerEvent{ID: s.values.ID(), SubjectID: saved.ID, Action: "import", Reason: "Reviewed Python manufacturer master transfer", Before: map[string]any{}, After: map[string]any{"code": saved.Code, "name": saved.Name, "data": saved.Data, "source_id": in.Source.ID, "source_fingerprint": fingerprint}, Actor: &p.ID}); e != nil {
			return e
		}
		out = &saved
		return nil
	})
	if e != nil {
		return nil, e
	}
	return out, nil
}
