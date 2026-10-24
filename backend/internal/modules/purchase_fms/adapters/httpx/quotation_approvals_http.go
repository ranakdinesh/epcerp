package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type quotationApprovalsHandler struct {
	s *application.QuotationApprovalsService
}

func quotationApprovalsFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "quotation_approvals.operation.failed", "Quotation approval operation failed")
}

func quotationApprovalsParams(r *http.Request) (string, string) {
	return chi.URLParam(r, "project"), chi.URLParam(r, "bo")
}
func (h quotationApprovalsHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Query()) != 0 {
		quotationApprovalsFail(w, r, domain.QuotationApprovalsInvalid("query", "No workspace query parameters are supported"))
		return
	}
	p, b := quotationApprovalsParams(r)
	v, e := h.s.Workspace(r.Context(), bearer(r), p, b)
	if !quotationApprovalsFail(w, r, e) {
		write(w, 200, v)
	}
}
func (h quotationApprovalsHandler) preview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if len(q) != 1 || len(q["quote_key"]) != 1 {
		quotationApprovalsFail(w, r, domain.QuotationApprovalsInvalid("quote_key", "Choose one saved quotation revision"))
		return
	}
	p, b := quotationApprovalsParams(r)
	v, e := h.s.Preview(r.Context(), bearer(r), p, b, q.Get("quote_key"))
	if !quotationApprovalsFail(w, r, e) {
		write(w, 200, v)
	}
}

type quotationApprovalsSubmitRequest struct {
	Proposals []domain.QuotationApprovalsNomination `json:"proposals"`
	Version   *int32                                `json:"version"`
	Reason    *string                               `json:"reason"`
	QuoteKey  *string                               `json:"quote_key"`
}
type quotationApprovalsReviewRequest struct {
	SelectedQuoteKey string                              `json:"selected_quote_key"`
	ReturnAction     string                              `json:"return_action"`
	Version          *int32                              `json:"version"`
	Reason           *string                             `json:"reason"`
	AttemptID        *string                             `json:"attempt_id"`
	Decisions        []domain.QuotationApprovalsDecision `json:"decisions"`
}

func quotationApprovalsDecode(w http.ResponseWriter, r *http.Request, action string) (domain.QuotationApprovalsCommand, error) {
	limit := int64(64 << 10)
	if action == "review" {
		limit = 1 << 20
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if e != nil {
		return domain.QuotationApprovalsCommand{}, e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	c := domain.QuotationApprovalsCommand{}
	if action == "submit" {
		var q quotationApprovalsSubmitRequest
		if e = d.Decode(&q); e != nil || q.Version == nil || q.Reason == nil || (q.QuoteKey == nil && len(q.Proposals) == 0) {
			return c, domain.QuotationApprovalsInvalid("", "Use version, reason and quote_key only")
		}
		c.Version = *q.Version
		c.Reason = *q.Reason
		if q.QuoteKey != nil {
			c.QuoteKey = *q.QuoteKey
		}
		c.Proposals = q.Proposals
	} else {
		var q quotationApprovalsReviewRequest
		if e = d.Decode(&q); e != nil || q.Version == nil || q.Reason == nil || q.AttemptID == nil || q.Decisions == nil {
			return c, domain.QuotationApprovalsInvalid("", "Use version, reason, attempt_id and item decisions only")
		}
		c.Version = *q.Version
		c.Reason = *q.Reason
		c.AttemptID = *q.AttemptID
		c.Decisions = q.Decisions
		c.SelectedQuoteKey = q.SelectedQuoteKey
		c.ReturnAction = q.ReturnAction
	}
	if e = d.Decode(&struct{}{}); e != io.EOF {
		return c, domain.QuotationApprovalsInvalid("", "Expected one quotation approval command")
	}
	return c.Validate(action)
}
func (h quotationApprovalsHandler) execute(w http.ResponseWriter, r *http.Request, action string) {
	q, e := quotationApprovalsDecode(w, r, action)
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
			return
		}
		quotationApprovalsFail(w, r, e)
		return
	}
	p, b := quotationApprovalsParams(r)
	v, e := h.s.Execute(r.Context(), bearer(r), p, b, action, q)
	if !quotationApprovalsFail(w, r, e) {
		logging.Ctx(r.Context()).Info().Str("event", "quotation_approvals.command.committed").Str("action", action).Str("bo_id", b).Msg("Quotation approval command committed")
		write(w, 200, v)
	}
}
func (h quotationApprovalsHandler) submit(w http.ResponseWriter, r *http.Request) {
	h.execute(w, r, "submit")
}
func (h quotationApprovalsHandler) review(w http.ResponseWriter, r *http.Request) {
	h.execute(w, r, "review")
}

func (h quotationApprovalsHandler) queue(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset := int64(0)
	var e error
	if len(q) > 0 {
		if len(q) != 1 || len(q["offset"]) != 1 {
			quotationApprovalsFail(w, r, domain.QuotationApprovalsInvalid("offset", "Use one offset only"))
			return
		}
		offset, e = strconv.ParseInt(q.Get("offset"), 10, 32)
		if e != nil {
			quotationApprovalsFail(w, r, domain.QuotationApprovalsInvalid("offset", "Use an integer offset"))
			return
		}
	}
	v, e := h.s.Queue(r.Context(), bearer(r), int32(offset))
	if !quotationApprovalsFail(w, r, e) {
		write(w, 200, v)
	}
}

func (h quotationApprovalsHandler) projectStatus(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Query()) != 0 {
		quotationApprovalsFail(w, r, domain.QuotationApprovalsInvalid("query", "No status query supported"))
		return
	}
	out, e := h.s.ProjectStatus(r.Context(), bearer(r), chi.URLParam(r, "project"))
	if !quotationApprovalsFail(w, r, e) {
		write(w, 200, out)
	}
}
