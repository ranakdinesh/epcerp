package dtos

import (
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func Key(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	return v.String()
}

func WorkspaceWorkspace(v sqlc.PurchaseFmsWorkspace) domain.Workspace {
	return domain.Workspace{ProjectID: v.ProjectID.String(), ManagerID: Key(v.PurchaseManagerID), Version: v.Version, Active: v.IsActive, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: Key(v.CreatedByID), UpdatedBy: Key(v.UpdatedByID)}
}

func WorkspaceHistory(v sqlc.PurchaseFmsOwnerHistory) domain.WorkspaceHistory {
	return domain.WorkspaceHistory{ID: v.ID.String(), ProjectID: v.ProjectID.String(), PreviousID: Key(v.PreviousManagerID), ManagerID: v.PurchaseManagerID.String(), ActorID: Key(v.CreatedByID), Version: v.WorkspaceVersion, Reason: v.Reason, CreatedAt: v.CreatedAt.Time}
}
