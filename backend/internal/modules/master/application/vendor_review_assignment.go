package application

import (
	"context"
	identity "github.com/ranakdinesh/epcERP/internal/identity"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type VendorReviewAssignmentService struct {
	u ports.VendorReviewAssignmentUnitOfWork
	a ports.Authorizer
	v ports.VendorDocumentValues
}

func NewVendorReviewAssignments(u ports.VendorReviewAssignmentUnitOfWork, a ports.Authorizer, v ports.VendorDocumentValues) *VendorReviewAssignmentService {
	return &VendorReviewAssignmentService{u, a, v}
}
func (s *VendorReviewAssignmentService) actor(ctx context.Context, p identity.Principal, users ports.VendorReviewAssignmentUsers, write bool) (identity.Principal, error) {
	actual, err := users.Get(ctx, p.ID)
	if err != nil {
		return p, err
	}
	if actual == nil || !actual.Active || !p.Active || p.ID == "" {
		return p, domain.VendorVerificationUnauthorized
	}
	for _, code := range []string{"accounts.vendor_verification.read"} {
		if !p.Allows(code) || !actual.Allows(code) {
			return p, domain.VendorVerificationForbidden
		}
	}
	if write && (!p.Allows("accounts.vendor_verification.assign") || !actual.Allows("accounts.vendor_verification.assign")) {
		return p, domain.VendorVerificationForbidden
	}
	return *actual, nil
}
func (s *VendorReviewAssignmentService) Get(ctx context.Context, token, supplier string) (result *domain.VendorReviewAssignment, err error) {
	principal, err := s.a.Resolve(ctx, token)
	if err != nil {
		return nil, err
	}

	err = s.u.Within(ctx, func(ctx context.Context, r ports.VendorReviewAssignmentRepository, u ports.VendorReviewAssignmentUsers) error {
		if _, err := s.actor(ctx, principal, u, false); err != nil {
			return err
		}
		ok, err := r.LockSupplier(ctx, supplier)
		if err != nil {
			return err
		}
		if !ok {
			return domain.VendorVerificationMissing
		}
		result, err = r.Get(ctx, supplier)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (s *VendorReviewAssignmentService) Options(ctx context.Context, token string) (result []domain.VendorReviewer, err error) {
	principal, err := s.a.Resolve(ctx, token)
	if err != nil {
		return nil, err
	}

	err = s.u.Within(ctx, func(ctx context.Context, r ports.VendorReviewAssignmentRepository, u ports.VendorReviewAssignmentUsers) error {
		if _, err := s.actor(ctx, principal, u, true); err != nil {
			return err
		}
		values, err := u.Candidates(ctx)
		if err != nil {
			return err
		}
		result = []domain.VendorReviewer{}
		for _, v := range values {
			if v.Eligible {
				result = append(result, v)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (s *VendorReviewAssignmentService) Save(ctx context.Context, token, supplier string, in domain.VendorReviewAssignmentInput) (result *domain.VendorReviewAssignment, err error) {
	principal, err := s.a.Resolve(ctx, token)
	if err != nil {
		return nil, err
	}

	err = s.u.Within(ctx, func(ctx context.Context, r ports.VendorReviewAssignmentRepository, u ports.VendorReviewAssignmentUsers) error {
		actor, err := s.actor(ctx, principal, u, true)
		if err != nil {
			return err
		}
		if err = domain.VendorReviewAssignmentValidate(in); err != nil {
			return err
		}
		ok, err := r.LockSupplier(ctx, supplier)
		if err != nil {
			return err
		}
		if !ok {
			return domain.VendorVerificationMissing
		}
		current, err := r.Get(ctx, supplier)
		if err != nil {
			return err
		}
		version := int32(0)
		id := s.v.ID()
		if current != nil {
			version = current.Version
			id = current.ID
		}
		if version != in.Version {
			return domain.VendorVerificationStale
		}
		if !in.Active && (current == nil || current.UserID != in.UserID) {
			return domain.VendorVerificationFieldError("user_id", "Revoke the current reviewer assignment")
		}
		if in.Active {
			reviewer, err := u.Get(ctx, in.UserID)
			if err != nil {
				return err
			}
			if reviewer == nil || !reviewer.Allows("accounts.vendor_verification.view") || !reviewer.Allows("accounts.vendor_verification.read") || !reviewer.Allows("accounts.vendor_verification.write") {
				return domain.VendorVerificationFieldError("user_id", "Choose an active Finance reviewer")
			}
		}
		result, err = r.Save(ctx, domain.VendorReviewAssignment{ID: id, SupplierID: supplier, UserID: in.UserID, Version: version + 1, Active: in.Active, CreatedBy: &actor.ID, UpdatedBy: &actor.ID}, in.Reason, s.v.ID())
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
