package dtos

import (
	"encoding/json"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func QuotationApprovalsRecord(v sqlc.QuotationApprovalSet) (domain.QuotationApprovalsRecord, error) {
	r := domain.QuotationApprovalsRecord{ProjectID: v.ProjectID.String(), BOID: v.BoID.String(), Version: v.Version, Active: v.IsActive, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: Actor(v.CreatedByID), UpdatedBy: Actor(v.UpdatedByID), Attempts: []domain.QuotationApprovalsAttempt{}}
	e := json.Unmarshal(v.Attempts, &r.Attempts)
	return r, e
}
