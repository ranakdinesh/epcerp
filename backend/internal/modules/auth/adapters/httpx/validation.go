package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx/dtos"
)

func validation(w http.ResponseWriter, field, message, kind string) {
	validationFailure(w, dtos.Invalid(field, message, kind))
}
func validationFailure(w http.ResponseWriter, err error) {
	var issue *dtos.Issue
	if errors.As(err, &issue) {
		write(w, 422, map[string]any{"detail": []dtos.Issue{*issue}})
	} else {
		write(w, 422, map[string]string{"detail": "Invalid request body"})
	}
}
func readJSON(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		media, _, err := mime.ParseMediaType(contentType)
		if err != nil || (media != "application/json" && !(strings.HasPrefix(media, "application/") && strings.HasSuffix(media, "+json"))) {
			validation(w, "", "JSON object required", "model_attributes_type")
			return nil, false
		}
	}
	body := http.MaxBytesReader(w, r.Body, 16384)
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			write(w, 413, map[string]string{"detail": "Request body is too large"})
		} else {
			validation(w, "", "Invalid request body", "json_invalid")
		}
		return nil, false
	}
	return raw, true
}
func requestBody[T any](w http.ResponseWriter, r *http.Request, parse func(io.Reader) (T, error)) (T, bool) {
	var zero T
	raw, ok := readJSON(w, r)
	if !ok {
		return zero, false
	}
	value, err := parse(bytes.NewReader(raw))
	if err != nil {
		validationFailure(w, err)
		return zero, false
	}
	return value, true
}
func decode(w http.ResponseWriter, r *http.Request, target any, allowed ...string) (map[string]json.RawMessage, bool) {
	raw, ok := readJSON(w, r)
	if !ok {
		return nil, false
	}
	object, err := dtos.Decode(bytes.NewReader(raw), target, allowed...)
	if err != nil {
		validationFailure(w, err)
		return nil, false
	}
	return object, true
}
