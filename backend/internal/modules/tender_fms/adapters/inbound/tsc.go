package inbound

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpinput"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"net/http"
)

func RegisterTSC(r chi.Router, s *application.TSC) {
	base := "/api/v1/tender-fms/drafts/{draft}/tsc"
	r.Get(base+"/options", func(w http.ResponseWriter, r *http.Request) {
		out, e := s.Options(r.Context(), captureToken(r), chi.URLParam(r, "draft"))
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		json.NewEncoder(w).Encode(out)
	})
	r.Post(base+"/{job}/answers/{question}/research", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Version int64  `json:"expected_version"`
			Query   string `json:"query"`
			Mode    string `json:"mode"`
		}
		if captureFailure(w, httpinput.Decode(w, r, 8192, &in)) {
			return
		}
		out, e := s.Research(r.Context(), captureToken(r), chi.URLParam(r, "draft"), chi.URLParam(r, "job"), chi.URLParam(r, "question"), in.Query, in.Mode, r.Header.Get("Idempotency-Key"), in.Version)
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		json.NewEncoder(w).Encode(out)
	})
	r.Get(base, func(w http.ResponseWriter, r *http.Request) {
		out, e := s.List(r.Context(), captureToken(r), chi.URLParam(r, "draft"))
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		json.NewEncoder(w).Encode(map[string]any{"items": out})
	})
	r.Post(base, func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Version int64  `json:"expected_version"`
			Mode    string `json:"mode"`
		}
		if captureFailure(w, httpinput.Decode(w, r, 8192, &in)) {
			return
		}
		out, e := s.Create(r.Context(), captureToken(r), chi.URLParam(r, "draft"), r.Header.Get("Idempotency-Key"), in.Mode, in.Version)
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(out)
	})
	r.Get(base+"/{job}", func(w http.ResponseWriter, r *http.Request) {
		out, e := s.Get(r.Context(), captureToken(r), chi.URLParam(r, "draft"), chi.URLParam(r, "job"))
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		json.NewEncoder(w).Encode(out)
	})
	r.Patch(base+"/{job}/answers/{question}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Version   int64                `json:"expected_version"`
			Status    string               `json:"status"`
			Answer    string               `json:"answer"`
			Citations []domain.TSCCitation `json:"citations"`
			Reason    string               `json:"reason"`
			Accepted  bool                 `json:"accepted"`
		}
		if captureFailure(w, httpinput.Decode(w, r, 32768, &in)) {
			return
		}
		out, e := s.Edit(r.Context(), captureToken(r), chi.URLParam(r, "draft"), chi.URLParam(r, "job"), in.Version, domain.TSCAnswer{QuestionID: chi.URLParam(r, "question"), Status: in.Status, Text: in.Answer, Citations: in.Citations, Accepted: in.Accepted}, in.Reason)
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		json.NewEncoder(w).Encode(out)
	})
	r.Get(base+"/{job}/document", func(w http.ResponseWriter, r *http.Request) {
		out, e := s.Export(r.Context(), captureToken(r), chi.URLParam(r, "draft"), chi.URLParam(r, "job"))
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		w.Header().Set("Content-Disposition", `attachment; filename="TSC-draft.docx"`)
		w.Write(out)
	})
	r.Get(base+"/{job}/evidence", func(w http.ResponseWriter, r *http.Request) {
		out, e := s.ExportEvidence(r.Context(), captureToken(r), chi.URLParam(r, "draft"), chi.URLParam(r, "job"))
		if captureFailure(w, e) {
			return
		}
		captureHeaders(w)
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="TSC-source-evidence.md"`)
		w.Write(out)
	})
}
