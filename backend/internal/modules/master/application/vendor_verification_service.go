package application

import (
	"context"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type VendorVerificationService struct {
	u masterPorts.VendorVerificationUnitOfWork
	a masterPorts.Authorizer
	v masterPorts.VendorVerificationValues
}

func NewVendorVerificationService(u masterPorts.VendorVerificationUnitOfWork, a masterPorts.Authorizer, v masterPorts.VendorVerificationValues) *VendorVerificationService {
	return &VendorVerificationService{u, a, v}
}
func (s *VendorVerificationService) actor(ctx context.Context, t string, write bool) (sharedIdentity.Principal, error) {
	p, e := s.a.Resolve(ctx, t)
	if e != nil {
		return p, e
	}
	if p.ID == "" || !p.Active {
		return p, masterDomain.VendorVerificationUnauthorized
	}
	if write {
		if !p.Allows("accounts.vendor_verification.read") || !p.Allows("accounts.vendor_verification.write") {
			return p, masterDomain.VendorVerificationForbidden
		}
	} else if !p.Allows("accounts.vendor_verification.read") && !p.Allows("settings.suppliers.read") {
		return p, masterDomain.VendorVerificationForbidden
	}
	return p, nil
}
func (s *VendorVerificationService) Status(ctx context.Context, t, id string) (masterDomain.VendorVerificationStatus, error) {
	p, e := s.actor(ctx, t, false)
	if e != nil {
		return masterDomain.VendorVerificationStatus{}, e
	}
	var result masterDomain.VendorVerificationStatus
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.VendorVerificationRepository, ev masterPorts.VendorVerificationEvidence, users masterPorts.VendorVerificationUsers) error {
		var err error
		p, err = vendorVerificationLive(ctx, users, p, false)
		if err != nil {
			return err
		}
		snapshot, e := ev.Snapshot(ctx, id)
		if e != nil {
			return e
		}
		if snapshot == nil {
			return masterDomain.VendorVerificationMissing
		}
		review, e := r.Get(ctx, id)
		if e != nil {
			return e
		}
		result = masterDomain.VendorVerificationEvaluate(*snapshot, review, s.v.Fingerprint(*snapshot))
		if !p.Allows("accounts.vendor_verification.read") {
			result.Review = nil
		}
		return nil
	})
	if e != nil {
		result = masterDomain.VendorVerificationStatus{}
	}
	return result, e
}
func (s *VendorVerificationService) Review(ctx context.Context, t, id string, c masterDomain.VendorVerificationCommand) (masterDomain.VendorVerificationStatus, error) {
	p, e := s.actor(ctx, t, true)
	if e != nil {
		return masterDomain.VendorVerificationStatus{}, e
	}
	var result masterDomain.VendorVerificationStatus
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.VendorVerificationRepository, ev masterPorts.VendorVerificationEvidence, users masterPorts.VendorVerificationUsers) error {
		var err error
		p, err = vendorVerificationLive(ctx, users, p, true)
		if err != nil {
			return err
		}
		snapshot, e := ev.Snapshot(ctx, id)
		if e != nil {
			return e
		}
		if snapshot == nil {
			return masterDomain.VendorVerificationMissing
		}
		previous, e := r.Get(ctx, id)
		if e != nil {
			return e
		}
		version := int32(0)
		if previous != nil {
			version = previous.Version
		}
		fingerprint := s.v.Fingerprint(*snapshot)
		if c.Version != version || c.Fingerprint != fingerprint {
			return masterDomain.VendorVerificationStale
		}
		if e = masterDomain.VendorVerificationValidate(*snapshot, c); e != nil {
			return e
		}
		review, e := r.Save(ctx, masterDomain.VendorVerificationReview{SupplierID: id, Version: version + 1, Fingerprint: fingerprint, Decisions: c.Decisions, Reason: c.Reason, UpdatedBy: &p.ID}, p.ID)
		if e != nil {
			return e
		}
		if e = r.Journal(ctx, *review, s.v.ID()); e != nil {
			return e
		}
		if workflow, ok := r.(masterPorts.DocumentApprovalLifecycle); ok {
			if e = workflow.Decided(ctx, *snapshot, c.Decisions, p.ID); e != nil {
				return e
			}
			snapshot, e = ev.Snapshot(ctx, id)
			if e != nil {
				return e
			}
		}
		result = masterDomain.VendorVerificationEvaluate(*snapshot, review, fingerprint)
		return nil
	})
	if e != nil {
		result = masterDomain.VendorVerificationStatus{}
	}
	return result, e
}

// Retain the resolved caller's ceiling and intersect it with current Auth truth.
// A stale token flag can never grant recovery or Finance authority.
func vendorVerificationLive(ctx context.Context, u masterPorts.VendorVerificationUsers, p sharedIdentity.Principal, write bool) (sharedIdentity.Principal, error) {
	actual, e := u.Get(ctx, p.ID)
	if e != nil {
		return sharedIdentity.Principal{}, e
	}
	if actual == nil || !actual.Active || actual.ID != p.ID {
		return sharedIdentity.Principal{}, masterDomain.VendorVerificationUnauthorized
	}
	permissions := []string{}
	for _, code := range p.Permissions {
		if actual.Allows(code) {
			permissions = append(permissions, code)
		}
	}
	p = sharedIdentity.Principal{ID: p.ID, Active: true, Superuser: p.Superuser && actual.Superuser, Permissions: permissions}
	if write {
		if !p.Allows("accounts.vendor_verification.read") || !p.Allows("accounts.vendor_verification.write") {
			return p, masterDomain.VendorVerificationForbidden
		}
	} else if !p.Allows("accounts.vendor_verification.read") && !p.Allows("settings.suppliers.read") {
		return p, masterDomain.VendorVerificationForbidden
	}
	return p, nil
}
