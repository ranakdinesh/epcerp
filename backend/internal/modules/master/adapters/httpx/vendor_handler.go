package httpx

import (
	"encoding/json"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	"io"
	"net/http"

	chi "github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type vendorHandler struct {
	s *masterApplication.VendorService
}

func vendorFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return failMaster(w, r, e, "suppliers.operation.failed")
}

func vendorDecode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	d.UseNumber()
	e := masterDTO.DecodeMasterJSON(d, v)
	if e == nil {
		var extra any
		if masterDTO.DecodeMasterJSON(d, &extra) != io.EOF {
			e = errors.New("trailing input")
		}
	}
	if e != nil {
		var max *http.MaxBytesError
		if errors.As(e, &max) {
			writeJSON(w, 413, map[string]string{"detail": "Request body too large"})
		} else {
			writeJSON(w, 422, map[string]string{"detail": "Invalid import payload"})
		}
		return false
	}
	return true
}
func RegisterVendorRoutes(r chi.Router, s *masterApplication.VendorService) {
	h := vendorHandler{s}
	r.Get("/api/v1/suppliers", h.list)
	r.Post("/api/v1/suppliers", h.create)
	r.Patch("/api/v1/suppliers/{supplier_id}", h.update)
	r.Post("/api/v1/suppliers/legacy-import", h.importRecord)
}
func (h vendorHandler) list(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.List(r.Context(), bearerToken(r))
	if !vendorFail(w, r, e) {
		writeJSON(w, 200, v)
	}
}
func (h vendorHandler) importRecord(w http.ResponseWriter, r *http.Request) {
	var in masterDomain.VendorInput
	if !vendorDecode(w, r, &in) {
		return
	}
	v, e := h.s.Import(r.Context(), bearerToken(r), in)
	if vendorFail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "suppliers.legacy_imported").Str("supplier_id", v.ID).Msg("Supplier imported")
	writeJSON(w, 200, v)
}

func (h vendorHandler) create(w http.ResponseWriter, r *http.Request) {
	var in masterDomain.VendorCreate
	if !vendorDecode(w, r, &in) {
		return
	}
	v, e := h.s.Create(r.Context(), bearerToken(r), in)
	if vendorFail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "suppliers.created").Str("supplier_id", v.ID).Msg("Supplier created")
	writeJSON(w, 201, v)
}
func (h vendorHandler) update(w http.ResponseWriter, r *http.Request) {
	var in masterDomain.VendorUpdate
	if !vendorDecode(w, r, &in) {
		return
	}
	v, e := h.s.Update(r.Context(), bearerToken(r), chi.URLParam(r, "supplier_id"), in)
	if vendorFail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "suppliers.updated").Str("supplier_id", v.ID).Msg("Supplier updated")
	writeJSON(w, 200, v)
}
