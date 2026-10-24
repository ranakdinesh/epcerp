package httpx

import (
	"encoding/json"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	"io"
	"net/http"
	"strings"

	chi "github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type itemHandler struct {
	s *masterApplication.ItemService
}

func RegisterItemRoutes(r chi.Router, s *masterApplication.ItemService) {
	h := itemHandler{s}
	r.Get("/api/v1/catalogue-items", h.list)
	r.Get("/api/v1/catalogue-items/references", h.references)
	r.Get("/api/v1/catalogue-items/{id}/history", h.history)
	r.Post("/api/v1/catalogue-items", h.create)
	r.Patch("/api/v1/catalogue-items/{id}", h.change)
}

func itemFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return failMaster(w, r, e, "items.operation.failed")
}

func itemDecode(w http.ResponseWriter, r *http.Request, v any) bool {

	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if e != nil {
		var big *http.MaxBytesError
		if errors.As(e, &big) {
			writeJSON(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			itemFail(w, r, masterDomain.ItemInvalid("", "Unable to read request body"))
		}
		return false
	}
	var fields map[string]json.RawMessage
	if masterDTO.UnmarshalMasterJSON(raw, &fields) != nil || fields == nil {
		itemFail(w, r, masterDomain.ItemInvalid("", "Provide a JSON object"))
		return false
	}
	for k, value := range fields {
		if strings.TrimSpace(string(value)) == "null" {
			itemFail(w, r, masterDomain.ItemInvalid(k, "Null is not accepted"))
			return false
		}
	}
	if specs, ok := fields["specifications"]; ok {
		var values map[string]json.RawMessage
		if masterDTO.UnmarshalMasterJSON(specs, &values) != nil || values == nil {
			itemFail(w, r, masterDomain.ItemInvalid("specifications", "Provide a specification object"))
			return false
		}
		for k, value := range values {
			var text string
			if strings.TrimSpace(string(value)) == "null" || masterDTO.UnmarshalMasterJSON(value, &text) != nil {
				itemFail(w, r, masterDomain.ItemInvalid(k, "Provide an option code as text"))
				return false
			}
		}
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if masterDTO.DecodeMasterJSON(d, v) != nil {
		itemFail(w, r, masterDomain.ItemInvalid("", "Provide supported fields with the correct value types"))
		return false
	}
	return true
}
func (h itemHandler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.List(r.Context(), bearerToken(r))
	if !itemFail(w, r, e) {
		writeJSON(w, 200, v)
	}
}
func (h itemHandler) references(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.References(r.Context(), bearerToken(r))
	if !itemFail(w, r, e) {
		writeJSON(w, 200, v)
	}
}
func (h itemHandler) history(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.History(r.Context(), bearerToken(r), chi.URLParam(r, "id"))
	if !itemFail(w, r, e) {
		writeJSON(w, 200, v)
	}
}
func itemLogChange(r *http.Request, v masterDomain.Item) {
	logging.Ctx(r.Context()).Info().Str("event", "items.saved").Str("item_id", v.ID).Int32("version", v.Version).Msg("Item operation committed")
}
func (h itemHandler) create(w http.ResponseWriter, r *http.Request) {
	var in masterDTO.ItemCreateRequest

	if !itemDecode(w, r, &in) {
		return
	}
	if in.Specifications == nil {
		itemFail(w, r, masterDomain.ItemInvalid("specifications", "Provide a specification object"))
		return
	}
	v, e := h.s.Create(r.Context(), bearerToken(r), in.ConfigurationID, in.ManufacturerID, in.Specifications, in.Model)
	if !itemFail(w, r, e) {
		itemLogChange(r, v)
		writeJSON(w, 201, v)
	}
}
func (h itemHandler) change(w http.ResponseWriter, r *http.Request) {
	var in masterDTO.ItemChangeRequest

	if !itemDecode(w, r, &in) {
		return
	}
	if in.Version < 1 {
		itemFail(w, r, masterDomain.ItemInvalid("version", "Provide a positive version"))
		return
	}
	v, e := h.s.Change(r.Context(), bearerToken(r), chi.URLParam(r, "id"), in.Status, in.Version, in.Reason)
	if !itemFail(w, r, e) {
		itemLogChange(r, v)
		writeJSON(w, 200, v)
	}
}
