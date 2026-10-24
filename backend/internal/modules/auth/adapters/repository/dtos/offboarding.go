package dtos

import (
	"encoding/json"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func Offboarding(v sqlc.AuthAuthOffboardingCase) (domain.OffboardingCase, error) {
	c := domain.OffboardingCase{ID: v.ID.String(), UserID: v.UserID.String(), ManagerID: v.ManagerID.String(), State: v.State, Reason: v.Reason, ProfileVersion: v.ProfileVersion, Version: v.Version, ApprovalByID: v.ApprovalByID.String(), CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedByID: v.CreatedByID.String(), UpdatedByID: v.UpdatedByID.String(), Items: []domain.HandoverItem{}}
	if v.ApprovedAt.Valid {
		at := v.ApprovedAt.Time
		c.ApprovedAt = &at
	}
	e := json.Unmarshal(v.Items, &c.Items)
	return c, e
}
