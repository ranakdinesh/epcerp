package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/logging"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type profileHandler struct{ s *application.ProfileService }

func profileFailed(w http.ResponseWriter, r *http.Request, e error) bool {
	if e == nil {
		return false
	}
	code := 500
	message := "Profile service unavailable"
	switch {
	case errors.Is(e, domain.ErrUnauthorized):
		code = 401
		message = "Authentication required"
	case errors.Is(e, domain.ErrForbidden):
		code = 403
		message = "Profile permission required"
	case errors.Is(e, domain.ErrUserMissing):
		code = 404
		message = "User unavailable"
	case errors.Is(e, domain.ErrProfileConflict):
		code = 409
		message = domain.ErrProfileConflict.Detail
	case errors.Is(e, domain.ErrProfileInvalid):
		code = 422
		message = "Use a display name of at most 100 characters or a PNG/JPEG up to 512KiB and 2048×2048 pixels"
	}
	var invalid *domain.Error
	if errors.As(e, &invalid) && invalid.Kind == domain.Invalid {
		code = 422
		message = invalid.Detail
	}
	if code == 500 {
		logging.Ctx(r.Context()).Error().Str("event", "auth.user_profile.operation.failed").Msg("Profile operation failed")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": message})
	return true
}
func profileRespond(w http.ResponseWriter, p domain.Profile) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(dtos.FromProfile(p))
}
func (h *profileHandler) me(w http.ResponseWriter, r *http.Request) {
	p, e := h.s.Get(r.Context(), bearer(r), "")
	if !profileFailed(w, r, e) {
		profileRespond(w, p)
	}
}
func profileValidID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, e := dtos.ProfileID(chi.URLParam(r, "user_id"))
	if e != nil {
		profileFailed(w, r, domain.ErrProfileInvalid)
		return "", false
	}
	return id, true
}
func (h *profileHandler) user(w http.ResponseWriter, r *http.Request) {
	id, ok := profileValidID(w, r)
	if !ok {
		return
	}
	p, e := h.s.Get(r.Context(), bearer(r), id)
	if !profileFailed(w, r, e) {
		profileRespond(w, p)
	}
}
func (h *profileHandler) update(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	p, e := dtos.ParseProfileUpdate(r.Body)
	if e != nil {
		profileFailed(w, r, domain.ErrProfileInvalid)
		return
	}
	var v domain.Profile
	if p.Contact != nil {
		v, e = h.s.UpdateDetails(r.Context(), bearer(r), p.Details())
	} else {
		v, e = h.s.Update(r.Context(), bearer(r), p.Name, nil, false)
	}
	if !profileFailed(w, r, e) {
		profileRespond(w, v)
	}
}
func (h *profileHandler) photo(w http.ResponseWriter, r *http.Request) {
	if ct := r.Header.Get("Content-Type"); ct != "image/png" && ct != "image/jpeg" {
		profileFailed(w, r, domain.ErrProfileInvalid)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512*1024)
	raw, e := io.ReadAll(r.Body)
	if e != nil {
		w.WriteHeader(413)
		return
	}
	if len(raw) == 0 {
		profileFailed(w, r, domain.ErrProfileInvalid)
		return
	}
	v, e := h.s.Update(r.Context(), bearer(r), nil, raw, true)
	if !profileFailed(w, r, e) {
		profileRespond(w, v)
	}
}
func (h *profileHandler) remove(w http.ResponseWriter, r *http.Request) {
	v, e := h.s.Update(r.Context(), bearer(r), nil, nil, true)
	if !profileFailed(w, r, e) {
		profileRespond(w, v)
	}
}
func (h *profileHandler) image(w http.ResponseWriter, r *http.Request) {
	id, ok := profileValidID(w, r)
	if !ok {
		return
	}
	p, e := h.s.Image(r.Context(), bearer(r), id)
	if profileFailed(w, r, e) {
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if len(p) == 0 {
		w.WriteHeader(204)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(p)
}
