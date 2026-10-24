package httpx

import (
	"github.com/go-chi/chi/v5"
	dto "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	app "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	"net/http"
)

func RegisterVendorReviewAssignments(r chi.Router, s *app.VendorReviewAssignmentService) {
	r.Get("/api/v1/suppliers/{supplier_id}/verification/assignment", func(w http.ResponseWriter, r *http.Request) {
		id, ok := vendorDocumentSupplier(w, r)
		if !ok {
			return
		}
		value, err := s.Get(r.Context(), bearerToken(r), id)
		if !vendorVerificationFail(w, r, err) {
			writeJSON(w, 200, map[string]any{"assignment": dto.VendorReviewAssignment(value)})
		}
	})
	r.Get("/api/v1/suppliers/{supplier_id}/verification/assignment/options", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := vendorDocumentSupplier(w, r); !ok {
			return
		}
		values, err := s.Options(r.Context(), bearerToken(r))
		if vendorVerificationFail(w, r, err) {
			return
		}
		result := []dto.VendorReviewerResponse{}
		for _, v := range values {
			result = append(result, dto.VendorReviewerResponse{ID: v.ID, Name: v.Name})
		}
		writeJSON(w, 200, result)
	})
	r.Put("/api/v1/suppliers/{supplier_id}/verification/assignment", func(w http.ResponseWriter, r *http.Request) {
		id, ok := vendorDocumentSupplier(w, r)
		if !ok {
			return
		}
		var request dto.VendorReviewAssignmentRequest
		if !vendorDocumentDecode(w, r, &request) {
			return
		}
		input, err := request.Domain()
		if vendorVerificationFail(w, r, err) {
			return
		}
		value, err := s.Save(r.Context(), bearerToken(r), id, input)
		if !vendorVerificationFail(w, r, err) {
			writeJSON(w, 200, dto.VendorReviewAssignment(value))
		}
	})
}
