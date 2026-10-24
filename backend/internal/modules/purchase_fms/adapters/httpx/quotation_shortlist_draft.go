package httpx

import (
	"encoding/json"
	"errors"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"io"
	"net/http"
)

func (h quotationApprovalsHandler) draft(w http.ResponseWriter, r *http.Request) {
	var input struct {
		GroupID   string                                `json:"group_id"`
		Version   *int32                                `json:"version"`
		Proposals []domain.QuotationApprovalsNomination `json:"proposals"`
		Comment   string                                `json:"comment"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	d.DisallowUnknownFields()
	e := d.Decode(&input)
	var large *http.MaxBytesError
	if errors.As(e, &large) {
		write(w, 413, map[string]string{"detail": "Request body too large"})
		return
	}
	if e != nil || input.Version == nil || d.Decode(&struct{}{}) != io.EOF {
		quotationApprovalsFail(w, r, domain.QuotationApprovalsInvalid("draft", "Use group_id, version, proposals and comment"))
		return
	}
	p, b := quotationApprovalsParams(r)
	out, e := h.s.SaveDraft(r.Context(), bearer(r), p, b, domain.QuotationShortlistDraft{GroupID: input.GroupID, Version: *input.Version, Proposals: input.Proposals, Comment: input.Comment})
	if !quotationApprovalsFail(w, r, e) {
		write(w, 200, out)
	}
}
