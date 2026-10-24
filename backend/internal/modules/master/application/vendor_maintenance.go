package application

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

func (s *VendorService) Create(ctx context.Context, t string, in masterDomain.VendorCreate) (*masterDomain.Vendor, error) {
	p, e := s.actor(ctx, t, "settings.suppliers.write")
	if e != nil {
		return nil, e
	}
	in.Code = masterDomain.VendorNormalizeCode(in.Code)
	in.Name = masterDomain.VendorNormalizeName(in.Name)
	if e = masterDomain.VendorValidateMaster(in.Code, in.Name, in.Data); e != nil {
		return nil, e
	}
	if e = masterDomain.VendorValidateReason(in.Reason); e != nil {
		return nil, e
	}
	var result *masterDomain.Vendor
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.VendorRepository) error {
		var e error
		result, e = r.Insert(ctx, masterDomain.Vendor{ID: s.v.ID(), Code: in.Code, Name: in.Name, Data: in.Data, Active: true, Version: 1, CreatedBy: &p.ID, UpdatedBy: &p.ID})
		if e != nil {
			return e
		}
		return r.Changed(ctx, *result, s.v.ID(), "created", in.Reason)
	})
	return result, e
}
func (s *VendorService) Update(ctx context.Context, t, id string, in masterDomain.VendorUpdate) (*masterDomain.Vendor, error) {
	p, e := s.actor(ctx, t, "settings.suppliers.write")
	if e != nil {
		return nil, e
	}
	if !masterDomain.VendorValidID(id) {
		return nil, masterDomain.VendorMissing
	}
	if e = masterDomain.VendorValidateReason(in.Reason); e != nil {
		return nil, e
	}
	var result *masterDomain.Vendor
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.VendorRepository) error {
		v, e := r.Get(ctx, id)
		if e != nil {
			return e
		}
		if v == nil {
			return masterDomain.VendorMissing
		}
		if in.Version != v.Version {
			return masterDomain.VendorStale
		}
		if e = masterDomain.VendorValidateMaster(v.Code, masterDomain.VendorNormalizeName(in.Name), in.Data); e != nil {
			return e
		}
		v.Name = masterDomain.VendorNormalizeName(in.Name)
		v.Data = in.Data
		v.Version++
		v.UpdatedBy = &p.ID
		if in.Active != nil {
			v.Active = *in.Active
		}
		result, e = r.Update(ctx, *v)
		if e != nil {
			return e
		}
		return r.Changed(ctx, *result, s.v.ID(), "updated", in.Reason)
	})
	return result, e
}
