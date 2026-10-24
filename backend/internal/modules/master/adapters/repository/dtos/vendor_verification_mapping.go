package dtos

import (
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func VendorVerificationFromRow(v db.SupplierVerification) (*masterDomain.VendorVerificationReview, error) {
	var decisions []masterDomain.VendorVerificationDecision
	if e := UnmarshalMasterJSON(v.Decisions, &decisions); e != nil {
		return nil, e
	}
	return &masterDomain.VendorVerificationReview{SupplierID: v.SupplierID.String(), Version: v.Version, Fingerprint: v.Fingerprint, Decisions: decisions, Reason: v.Reason, UpdatedAt: v.UpdatedAt.Time, UpdatedBy: Actor(v.UpdatedByID)}, nil
}
