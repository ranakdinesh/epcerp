package dtos

import (
	"encoding/json"

	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func QuotationsRecord(row sqlc.PurchaseGroupQuotation) (domain.QuotationsRecord, error) {
	r := domain.QuotationsRecord{BOID: row.BoID.String(), ProjectID: row.ProjectID.String(), Version: row.Version, Active: row.IsActive, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time, CreatedBy: Actor(row.CreatedByID), UpdatedBy: Actor(row.UpdatedByID)}
	e := json.Unmarshal(row.Quotes, &r.Quotes)
	return r, e
}

func QuotationsAttachment(row sqlc.PurchaseQuotationAttachment) domain.QuotationsAttachment {
	return domain.QuotationsAttachment{ID: row.ID.String(), DocumentID: row.DocumentID.String(), Filename: row.Filename, ContentType: row.ContentType, ByteSize: row.ByteSize, SHA256: row.Sha256, Active: row.IsActive, CreatedAt: row.CreatedAt.Time, CreatedBy: row.CreatedByID.String(), Ciphertext: row.Ciphertext}
}
