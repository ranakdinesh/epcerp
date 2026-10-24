package httpx

import (
	"encoding/json"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	"io"
	"net/http"
	"regexp"

	chi "github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type vendorVerificationHandler struct {
	s *masterApplication.VendorVerificationService
}

var vendorVerificationUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func vendorVerificationFail(w http.ResponseWriter, r *http.Request, e error) bool {
	return failMaster(w, r, e, "supplier_verification.failed")
}

func RegisterVendorVerificationRoutes(r chi.Router, s *masterApplication.VendorVerificationService) {
	h := vendorVerificationHandler{s}
	r.Get("/api/v1/suppliers/{supplier_id}/verification", h.status)
	r.Post("/api/v1/suppliers/{supplier_id}/verification", h.review)
}
func (h vendorVerificationHandler) status(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "supplier_id")
	if !vendorVerificationUuid.MatchString(id) {
		writeJSON(w, 422, map[string]string{"detail": "Invalid supplier identity"})
		return
	}
	v, e := h.s.Status(r.Context(), bearerToken(r), id)
	if !vendorVerificationFail(w, r, e) {
		writeJSON(w, 200, v)
	}
}
func (h vendorVerificationHandler) review(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "supplier_id")
	if !vendorVerificationUuid.MatchString(id) {
		writeJSON(w, 422, map[string]string{"detail": "Invalid supplier identity"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	var in masterDomain.VendorVerificationCommand
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	e := masterDTO.DecodeMasterJSON(decoder, &in)
	var extra any
	if e == nil && masterDTO.DecodeMasterJSON(decoder, &extra) != io.EOF {
		e = errors.New("trailing input")
	}
	if e != nil {
		var maximum *http.MaxBytesError
		if errors.As(e, &maximum) {
			writeJSON(w, 413, map[string]string{"detail": "Request too large"})
		} else {
			writeJSON(w, 422, map[string]string{"detail": "Invalid review payload"})
		}
		return
	}
	v, e := h.s.Review(r.Context(), bearerToken(r), id, in)
	if vendorVerificationFail(w, r, e) {
		return
	}
	logging.Ctx(r.Context()).Info().Str("event", "supplier_verification.reviewed").Str("supplier_id", id).Msg("Finance review recorded")
	writeJSON(w, 200, v)
}
