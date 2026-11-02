package inbound

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpinput"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"net/http"
)

func RegisterTSCFormats(r chi.Router, s *application.TSCFormats) {
	base := "/api/v1/settings/tsc-formats"
	r.Get(base, func(w http.ResponseWriter, r *http.Request) {
		out, e := s.List(r.Context(), captureToken(r))
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		json.NewEncoder(w).Encode(map[string]any{"items": out})
	})
	r.Get(base+"/{format}", func(w http.ResponseWriter, r *http.Request) {
		out, e := s.Get(r.Context(), captureToken(r), chi.URLParam(r, "format"))
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		json.NewEncoder(w).Encode(out)
	})
	for _, kind := range []string{"preview.pdf", "template.docx"} {
		r.Get(base+"/{format}/"+kind, func(w http.ResponseWriter, r *http.Request) {
			pdf := kind == "preview.pdf"
			data, err := s.Preview(r.Context(), captureToken(r), chi.URLParam(r, "format"), pdf)
			if captureFailure(w, err) {
				return
			}
			captureHeaders(w)
			if pdf {
				w.Header().Set("Content-Type", "application/pdf")
				w.Header().Set("Content-Disposition", `inline; filename="TSC-format.pdf"`)
			} else {
				w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
				w.Header().Set("Content-Disposition", `attachment; filename="TSC-format.docx"`)
			}
			w.Write(data)
		})
	}
	save := func(w http.ResponseWriter, r *http.Request) {
		var in domain.TSCFormatInput
		if captureFailure(w, httpinput.Decode(w, r, 512<<10, &in)) {
			return
		}
		out, e := s.Save(r.Context(), captureToken(r), chi.URLParam(r, "format"), in)
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		if r.Method == http.MethodPost {
			w.WriteHeader(201)
		}
		json.NewEncoder(w).Encode(out)
	}
	r.Post(base, save)
	r.Patch(base+"/{format}", save)
}
