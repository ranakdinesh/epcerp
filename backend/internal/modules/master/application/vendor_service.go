package application

import (
	"context"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type VendorService struct {
	u masterPorts.VendorUnitOfWork
	a masterPorts.Authorizer
	v masterPorts.VendorValues
}

func NewVendorService(u masterPorts.VendorUnitOfWork, a masterPorts.Authorizer, v masterPorts.VendorValues) *VendorService {
	return &VendorService{u, a, v}
}
func (s *VendorService) actor(ctx context.Context, t, permission string) (sharedIdentity.Principal, error) {
	p, e := s.a.Resolve(ctx, t)
	if e != nil {
		return p, e
	}
	if p.ID == "" || !p.Active {
		return p, masterDomain.VendorUnauthorized
	}
	if !p.Allows(permission) && !(permission == "settings.suppliers.read" && p.Allows("accounts.vendor_verification.read")) {
		return p, masterDomain.VendorForbidden
	}
	return p, nil
}
func (s *VendorService) List(ctx context.Context, t string) ([]masterDomain.Vendor, error) {
	if _, e := s.actor(ctx, t, "settings.suppliers.read"); e != nil {
		return nil, e
	}
	var result []masterDomain.Vendor
	e := s.u.Within(ctx, func(ctx context.Context, r masterPorts.VendorRepository) error {
		var e error
		result, e = r.List(ctx)
		return e
	})
	return result, e
}
func (s *VendorService) Import(ctx context.Context, t string, in masterDomain.VendorInput) (*masterDomain.Vendor, error) {
	p, e := s.actor(ctx, t, "settings.suppliers.import")
	if e != nil {
		return nil, e
	}
	if e = masterDomain.VendorValidate(in); e != nil {
		return nil, e
	}
	var result *masterDomain.Vendor
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.VendorRepository) error {
		existing, e := r.BySource(ctx, in.Source.ID)
		if e != nil {
			return e
		}
		if existing != nil {
			if existing.Code != in.Code || existing.Name != in.Name || existing.Active != *in.Active || existing.Source.Version != in.Source.Version || !existing.Source.CreatedAt.Equal(in.Source.CreatedAt) || !existing.Source.UpdatedAt.Equal(in.Source.UpdatedAt) || s.v.Fingerprint(existing.Data) != s.v.Fingerprint(in.Data) {
				return masterDomain.VendorConflict
			}
			result = existing
			return nil
		}
		v := masterDomain.Vendor{ID: s.v.ID(), Code: in.Code, Name: in.Name, Data: in.Data, Active: *in.Active, Version: 1, CreatedBy: &p.ID, UpdatedBy: &p.ID, Source: in.Source}
		result, e = r.Insert(ctx, v)
		if e != nil {
			return e
		}
		return r.Imported(ctx, result.ID, s.v.ID(), p.ID)
	})
	return result, e
}
