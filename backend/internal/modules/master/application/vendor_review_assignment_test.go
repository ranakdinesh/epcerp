package application

import (
	"context"
	"errors"
	identity "github.com/ranakdinesh/epcERP/internal/identity"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
	"testing"
)

type reviewerAssignmentUsers struct{ actual identity.Principal }

func (u reviewerAssignmentUsers) Get(context.Context, string) (*identity.Principal, error) {
	return &u.actual, nil
}
func (u reviewerAssignmentUsers) Candidates(context.Context) ([]domain.VendorReviewer, error) {
	return []domain.VendorReviewer{{ID: "eligible", Name: "Finance", Eligible: true}, {ID: "ineligible", Eligible: false}}, nil
}

type reviewerAssignmentUow struct {
	users reviewerAssignmentUsers
	fail  bool
}

func (u reviewerAssignmentUow) Within(ctx context.Context, work func(context.Context, ports.VendorReviewAssignmentRepository, ports.VendorReviewAssignmentUsers) error) error {
	if err := work(context.WithValue(ctx, reviewAssignmentTransaction{}, true), nil, u.users); err != nil {
		return err
	}
	if u.fail {
		return errors.New("commit failed")
	}
	return nil
}

type reviewerAssignmentAuth struct{ actor identity.Principal }

type reviewAssignmentTransaction struct{}

func (a reviewerAssignmentAuth) Resolve(ctx context.Context, _ string) (identity.Principal, error) {
	if ctx.Value(reviewAssignmentTransaction{}) != nil {
		return identity.Principal{}, errors.New("authentication must resolve before the assignment transaction")
	}
	return a.actor, nil
}
func TestReviewerOptionsRequireLiveAssignmentAuthorityAndCommit(t *testing.T) {
	super := identity.Principal{ID: "admin", Active: true, Superuser: true}
	service := NewVendorReviewAssignments(reviewerAssignmentUow{users: reviewerAssignmentUsers{super}}, reviewerAssignmentAuth{super}, nil)
	values, err := service.Options(t.Context(), "token")
	if err != nil || len(values) != 1 || values[0].ID != "eligible" {
		t.Fatal("eligible-only reviewer options", err)
	}
	stale := super
	stale.Superuser = false
	stale.Permissions = []string{"accounts.vendor_verification.read"}
	service.u = reviewerAssignmentUow{users: reviewerAssignmentUsers{stale}}
	if _, err = service.Options(t.Context(), "old-token"); !errors.Is(err, domain.VendorVerificationForbidden) {
		t.Fatal("stale superuser bypassed live authority", err)
	}
	service.u = reviewerAssignmentUow{users: reviewerAssignmentUsers{super}, fail: true}
	values, err = service.Options(t.Context(), "token")
	if err == nil || values != nil {
		t.Fatal("uncommitted options released")
	}
}
