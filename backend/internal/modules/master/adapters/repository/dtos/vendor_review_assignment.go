package dtos

import (
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func VendorReviewAssignment(v db.VendorReviewAssignmentGetRow) *domain.VendorReviewAssignment {
	return &domain.VendorReviewAssignment{ID: v.ID.String(), SupplierID: v.SupplierID.String(), UserID: v.UserID.String(), ReviewerName: v.Username, Version: v.Version, Active: v.IsActive, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: Actor(v.CreatedByID), UpdatedBy: Actor(v.UpdatedByID)}
}

func VendorReviewAssignmentSnapshot(v *domain.VendorReviewAssignment) ([]byte, error) {
	return MarshalMasterJSON(map[string]any{"id": v.ID, "supplier_id": v.SupplierID, "user_id": v.UserID, "version": v.Version, "is_active": v.Active, "created_at": v.CreatedAt, "updated_at": v.UpdatedAt, "created_by_id": v.CreatedBy, "updated_by_id": v.UpdatedBy})
}
