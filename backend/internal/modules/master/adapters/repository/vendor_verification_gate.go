package repository

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"

	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type VendorVerificationGate struct {
	e *VendorVerificationEvidenceBridge
	r *vendorVerificationRepository
	c database.Connection
}

// c must be the caller's transaction: the provider locks the supplier while checking current evidence.
func NewVendorVerificationGate(c database.Connection, s masterContracts.VendorDirectory, d masterContracts.VendorDocumentReader) *VendorVerificationGate {
	return &VendorVerificationGate{&VendorVerificationEvidenceBridge{s, d}, &vendorVerificationRepository{q: NewStore(c).Queries}, c}
}
func (g *VendorVerificationGate) status(ctx context.Context, id string) (masterDomain.VendorVerificationStatus, error) {
	if !database.IsTransactionConnection(ctx, g.c) {
		return masterDomain.VendorVerificationStatus{}, masterDomain.VendorVerificationErrTransactionRequired
	}
	s, e := g.e.Snapshot(ctx, id)
	if e != nil {
		return masterDomain.VendorVerificationStatus{}, e
	}
	if s == nil {
		return masterDomain.VendorVerificationStatus{}, masterDomain.VendorVerificationMissing
	}
	r, e := g.r.Get(ctx, id)
	if e != nil {
		return masterDomain.VendorVerificationStatus{}, e
	}
	return masterDomain.VendorVerificationEvaluate(*s, r, (VendorVerificationValues{}).Fingerprint(*s)), nil
}
func (g *VendorVerificationGate) Check(ctx context.Context, id string) (masterContracts.VendorVerificationEligibility, error) {
	s, e := g.status(ctx, id)
	return masterContracts.VendorVerificationEligibility{CanInviteQuotation: s.CanInvite, CanGeneratePurchaseOrder: s.CanOrder, Message: s.Message}, e
}
func (g *VendorVerificationGate) RequirePurchaseOrder(ctx context.Context, id string) error {
	s, e := g.status(ctx, id)
	if e != nil {
		return e
	}
	return masterDomain.VendorVerificationRequirePurchaseOrder(s)
}

func (g *VendorVerificationGate) Source(ctx context.Context, id string) (masterContracts.VendorVerificationSourceStatus, error) {
	s, e := g.status(ctx, id)
	if e != nil {
		return masterContracts.VendorVerificationSourceStatus{}, e
	}
	return masterContracts.VendorVerificationSourceStatus{State: s.State, Fingerprint: s.Fingerprint, Message: s.Message}, nil
}
