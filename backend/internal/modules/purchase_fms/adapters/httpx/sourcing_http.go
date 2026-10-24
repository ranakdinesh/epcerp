package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	application "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type sourcingHandler struct{ s *application.SourcingService }

func sourcingFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return fail(w, r, e, "purchase_sourcing.operation.failed", "Sourcing operation failed")
}

func sourcingNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if len(r.URL.Query()) > 0 {
		sourcingFail(w, r, domain.SourcingInvalid("query", "No query parameters are supported"))
		return false
	}
	return true
}
func (h sourcingHandler) workspace(w http.ResponseWriter, r *http.Request) {
	if !sourcingNoQuery(w, r) {
		return
	}
	v, e := h.s.Workspace(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"))
	if !sourcingFail(w, r, e) {
		write(w, 200, v)
	}
}

var sourcingFields = map[string][]string{"configure": {"approver_id"}, "rfq": {"offer_id", "supplier_id", "due_date", "rfq_terms"}, "dispatch": {"offer_id", "sent_date", "channel", "dispatch_reference"}, "no_bid": {"offer_id"}, "quote": {"offer_id", "quote_reference", "evidence_reference", "valid_until", "pricing_mode", "line_total", "unit_price", "discount", "tax", "freight", "other", "delivery_days", "payment_terms", "warranty", "compliant", "compliance_notes"}, "propose": {"offer_id", "loi_terms"}, "decide": {"approve"}, "reopen": {}, "release": {"sent_date", "channel", "dispatch_reference"}}

func sourcingDecode(w http.ResponseWriter, r *http.Request) (domain.SourcingCommand, bool) {
	var q domain.SourcingCommand
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			write(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			write(w, 422, map[string]string{"detail": "Invalid command body"})
		}
		return q, false
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		sourcingFail(w, r, domain.SourcingInvalid("body", "Expected a command object"))
		return q, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		sourcingFail(w, r, domain.SourcingInvalid("body", "Use the documented command fields and types"))
		return q, false
	}
	if e = decoder.Decode(&struct{}{}); e != io.EOF {
		sourcingFail(w, r, domain.SourcingInvalid("body", "Expected one command object"))
		return q, false
	}
	// Commands are flat objects; reject duplicate keys rather than accepting an ambiguous approval.
	scanner := json.NewDecoder(bytes.NewReader(raw))
	start, scanErr := scanner.Token()
	if scanErr != nil || start != json.Delim('{') {
		sourcingFail(w, r, domain.SourcingInvalid("body", "Expected a command object"))
		return q, false
	}
	seen := map[string]bool{}
	for scanner.More() {
		bearer, err := scanner.Token()
		key, valid := bearer.(string)
		if err != nil || !valid || seen[key] {
			sourcingFail(w, r, domain.SourcingInvalid("body", "Command fields must occur once"))
			return q, false
		}
		seen[key] = true
		var value json.RawMessage
		if err = scanner.Decode(&value); err != nil {
			sourcingFail(w, r, domain.SourcingInvalid("body", "Invalid command value"))
			return q, false
		}
	}
	var keys map[string]json.RawMessage
	if e = json.Unmarshal(raw, &keys); e != nil {
		sourcingFail(w, r, domain.SourcingInvalid("body", "Expected a command object"))
		return q, false
	}
	for _, key := range []string{"action", "version", "reason"} {
		if _, ok := keys[key]; !ok || bytes.Equal(bytes.TrimSpace(keys[key]), []byte("null")) {
			sourcingFail(w, r, domain.SourcingInvalid(key, "Provide the required command field"))
			return q, false
		}
	}
	allowed, ok := sourcingFields[q.Action]
	if !ok {
		sourcingFail(w, r, domain.SourcingInvalid("action", "Choose a sourcing action"))
		return q, false
	}
	for key := range keys {
		if key == "action" || key == "version" || key == "reason" {
			continue
		}
		found := false
		for _, v := range allowed {
			found = found || v == key
		}
		if !found {
			sourcingFail(w, r, domain.SourcingInvalid(key, "Field is not permitted for this sourcing action"))
			return q, false
		}
	}
	q, e = q.Validate()
	if sourcingFail(w, r, e) {
		return q, false
	}
	return q, true
}
func (h sourcingHandler) execute(w http.ResponseWriter, r *http.Request) {
	if !sourcingNoQuery(w, r) {
		return
	}
	q, ok := sourcingDecode(w, r)
	if !ok {
		return
	}
	v, e := h.s.Execute(r.Context(), bearer(r), chi.URLParam(r, "project"), chi.URLParam(r, "package"), q)
	if sourcingFail(w, r, e) {
		return
	}
	if v.Case != nil {
		logging.Ctx(r.Context()).Info().Str("event", "purchase_sourcing.command.saved").Str("subject_id", v.Case.ID).Str("action", q.Action).Int32("version", v.Case.Version).Msg("Sourcing command saved")
	}
	write(w, 200, v)
}
