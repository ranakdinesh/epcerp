package dtos

import (
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func VendorDocumentFromRow(v db.SupplierDocument) *masterDomain.VendorDocumentDocument {
	return &masterDomain.VendorDocumentDocument{Version: v.Version, ID: v.ID.String(), SupplierID: v.SupplierID.String(), DocType: v.DocType, Title: v.Title, Filename: v.OriginalFilename, ContentType: v.ContentType, ByteSize: v.ByteSize, SHA256: v.Sha256, Active: v.IsActive, Encrypted: true, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time, CreatedBy: Actor(v.CreatedByID), UpdatedBy: Actor(v.UpdatedByID), Ciphertext: v.EncryptedContent, ObjectBucket: v.ObjectBucket.String, ObjectKey: v.ObjectKey.String, Source: masterDomain.VendorDocumentSource{ID: v.SourceID.String(), CreatedAt: v.SourceCreatedAt.Time, UpdatedAt: v.SourceUpdatedAt.Time}}
}
