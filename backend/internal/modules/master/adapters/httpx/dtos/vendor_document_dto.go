package dtos

import (
	"time"

	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type VendorDocumentDocumentDTO struct {
	ApprovalState   string    `json:"approval_state"`
	ApprovalComment string    `json:"approval_comment"`
	Version         int32     `json:"version"`
	ID              string    `json:"id"`
	SupplierID      string    `json:"supplier_id"`
	DocType         string    `json:"doc_type"`
	Title           string    `json:"title"`
	Filename        string    `json:"original_filename"`
	ContentType     string    `json:"content_type"`
	ByteSize        int32     `json:"byte_size"`
	SHA256          string    `json:"sha256"`
	Active          bool      `json:"is_active"`
	Encrypted       bool      `json:"is_encrypted"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedBy       *string   `json:"created_by_id"`
	UpdatedBy       *string   `json:"updated_by_id"`
}

func VendorDocumentDocumentDTOFromDomain(v domain.VendorDocumentDocument) VendorDocumentDocumentDTO {
	return VendorDocumentDocumentDTO{ApprovalState: v.ApprovalState, ApprovalComment: v.ApprovalComment,
		Version:     v.Version,
		ID:          v.ID,
		SupplierID:  v.SupplierID,
		DocType:     v.DocType,
		Title:       v.Title,
		Filename:    v.Filename,
		ContentType: v.ContentType,
		ByteSize:    v.ByteSize,
		SHA256:      v.SHA256,
		Active:      v.Active,
		Encrypted:   v.Encrypted,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
		CreatedBy:   v.CreatedBy,
		UpdatedBy:   v.UpdatedBy,
	}
}
func (v VendorDocumentDocumentDTO) Domain() domain.VendorDocumentDocument {
	return domain.VendorDocumentDocument{ApprovalState: v.ApprovalState, ApprovalComment: v.ApprovalComment,
		Version:     v.Version,
		ID:          v.ID,
		SupplierID:  v.SupplierID,
		DocType:     v.DocType,
		Title:       v.Title,
		Filename:    v.Filename,
		ContentType: v.ContentType,
		ByteSize:    v.ByteSize,
		SHA256:      v.SHA256,
		Active:      v.Active,
		Encrypted:   v.Encrypted,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
		CreatedBy:   v.CreatedBy,
		UpdatedBy:   v.UpdatedBy,
	}
}

type VendorDocumentInputDTO struct {
	Source      VendorDocumentSourceDTO `json:"source"`
	DocType     string                  `json:"doc_type"`
	Title       string                  `json:"title"`
	Filename    string                  `json:"original_filename"`
	ContentType string                  `json:"content_type"`
	Content     []byte                  `json:"content"`
	Active      *bool                   `json:"is_active"`
}

func VendorDocumentInputDTOFromDomain(v domain.VendorDocumentInput) VendorDocumentInputDTO {
	return VendorDocumentInputDTO{
		Source:      VendorDocumentSourceDTOFromDomain(v.Source),
		DocType:     v.DocType,
		Title:       v.Title,
		Filename:    v.Filename,
		ContentType: v.ContentType,
		Content:     v.Content,
		Active:      v.Active,
	}
}
func (v VendorDocumentInputDTO) Domain() domain.VendorDocumentInput {
	return domain.VendorDocumentInput{
		Source:      v.Source.Domain(),
		DocType:     v.DocType,
		Title:       v.Title,
		Filename:    v.Filename,
		ContentType: v.ContentType,
		Content:     v.Content,
		Active:      v.Active,
	}
}

type VendorDocumentSourceDTO struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func VendorDocumentSourceDTOFromDomain(v domain.VendorDocumentSource) VendorDocumentSourceDTO {
	return VendorDocumentSourceDTO{
		ID:        v.ID,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}
func (v VendorDocumentSourceDTO) Domain() domain.VendorDocumentSource {
	return domain.VendorDocumentSource{
		ID:        v.ID,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}
