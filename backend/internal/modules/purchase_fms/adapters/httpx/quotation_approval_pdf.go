package httpx

import (
	"encoding/json"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"io"
	"net/http"
	"strings"
)

func (h quotationApprovalsHandler) print(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if len(q["group_id"]) != 1 || len(q) > 2 || (len(q) == 2 && len(q["attempt_id"]) != 1 && len(q["quote_keys"]) != 1) {
		quotationApprovalsFail(w, r, domain.QuotationApprovalsInvalid("group_id", "Choose a group and optional frozen submission"))
		return
	}
	nominations := []domain.QuotationApprovalsNomination{}
	financeNotes := map[string]string{}
	if r.Method == "POST" {
		var input struct {
			Proposals    []domain.QuotationApprovalsNomination `json:"proposals"`
			FinanceNotes map[string]string                     `json:"finance_notes"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		d.DisallowUnknownFields()
		if e := d.Decode(&input); e != nil || d.Decode(&struct{}{}) != io.EOF || (len(input.Proposals) == 0 && len(input.FinanceNotes) == 0) || (len(input.Proposals) > 0 && q.Get("attempt_id") != "") || (len(input.FinanceNotes) > 0 && (len(input.Proposals) > 0 || q.Get("attempt_id") == "")) {
			quotationApprovalsFail(w, r, domain.QuotationApprovalsInvalid("proposals", "Print one complete draft shortlist"))
			return
		}
		nominations = input.Proposals
		financeNotes = input.FinanceNotes
	}
	p, b := quotationApprovalsParams(r)
	data, e := h.s.Print(r.Context(), bearer(r), p, b, q.Get("group_id"), q.Get("attempt_id"), nominations, func() []string {
		if q.Get("quote_keys") == "" {
			return nil
		}
		return strings.Split(q.Get("quote_keys"), ",")
	}(), financeNotes)
	if quotationApprovalsFail(w, r, e) {
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="quotation-comparison.pdf"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
