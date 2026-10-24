package repository

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"
	"io/fs"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type VendorDocumentEvidenceReader struct {
	q       *db.Queries
	c       masterPorts.VendorDocumentCipher
	i       masterPorts.VendorDocumentInspector
	objects masterPorts.VendorDocumentObjects
}

func NewVendorDocumentEvidenceReader(c database.Connection, cipher masterPorts.VendorDocumentCipher, i masterPorts.VendorDocumentInspector) *VendorDocumentEvidenceReader {
	return &VendorDocumentEvidenceReader{q: NewStore(c).Queries, c: cipher, i: i}
}
func (r *VendorDocumentEvidenceReader) Active(ctx context.Context, id string) ([]masterContracts.VendorDocumentEvidence, error) {
	rows, e := r.q.VendorDocumentsActiveEvidence(ctx, masterDTO.UUID(id))
	if e != nil {
		return nil, e
	}
	result := []masterContracts.VendorDocumentEvidence{}
	for _, v := range rows {
		previewable, err := r.previewable(ctx, v)
		if err != nil {
			return nil, err
		}
		approval, approvalErr := r.q.DocumentApprovalState(ctx, db.DocumentApprovalStateParams{DocumentID: v.ID, DocumentVersion: v.Version, Sha256: v.Sha256})
		if approvalErr != nil && !errors.Is(approvalErr, pgx.ErrNoRows) {
			return nil, approvalErr
		}
		result = append(result, masterContracts.VendorDocumentEvidence{ID: v.ID.String(), DocType: v.DocType, SHA256: v.Sha256, Version: v.Version, Previewable: previewable, ApprovalKnown: approvalErr == nil, Approval: approval.State, ApprovalComment: approval.Comment})
	}
	return result, nil
}

func (r *VendorDocumentEvidenceReader) previewable(ctx context.Context, v db.SupplierDocument) (bool, error) {
	encrypted := v.EncryptedContent
	if v.ObjectKey.Valid {
		if r.objects == nil {
			return false, errors.New("document object storage unavailable")
		}
		var err error
		encrypted, err = r.objects.Get(ctx, v.ObjectBucket.String, v.ObjectKey.String)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}
	data, e := r.c.Open(encrypted, v.Sha256)
	if e != nil {
		return false, e
	}
	_, e = r.i.Inspect(data)
	return e == nil, nil
}

func (r *VendorDocumentEvidenceReader) WithObjects(o masterPorts.VendorDocumentObjects) *VendorDocumentEvidenceReader {
	r.objects = o
	return r
}
