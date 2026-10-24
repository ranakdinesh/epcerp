// Package httpinput validates bounded closed JSON without duplicate keys.
package httpinput

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

type Error struct {
	Status int
	Detail string
}

func (e *Error) Error() string { return e.Detail }
func Decode(w http.ResponseWriter, r *http.Request, max int64, out any) error {
	if !strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]), "application/json") {
		return &Error{415, "Use application/json"}
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, max))
	if e != nil {
		var limit *http.MaxBytesError
		if errors.As(e, &limit) {
			return &Error{413, "Request body exceeds its limit"}
		}
		return &Error{400, "Cannot read JSON request"}
	}
	if !utf8.Valid(b) {
		return &Error{400, "Request must contain valid UTF-8"}
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if e = object(d, 0); e != nil {
		return &Error{400, "Malformed JSON, duplicate keys or excessive nesting"}
	}
	if _, e = d.Token(); e != io.EOF {
		return &Error{400, "Request must contain exactly one JSON value"}
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(out); e != nil {
		return &Error{422, "Unknown, missing or incorrectly typed request fields"}
	}
	return nil
}
func object(d *json.Decoder, depth int) error {
	if depth > 30 {
		return errors.New("nesting")
	}
	t, e := d.Token()
	if e != nil {
		return e
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			t, e = d.Token()
			if e != nil {
				return e
			}
			key, ok := t.(string)
			if !ok || seen[key] {
				return errors.New("duplicate key")
			}
			seen[key] = true
			if e = object(d, depth+1); e != nil {
				return e
			}
		}
		t, e = d.Token()
		if e != nil || t != json.Delim('}') {
			return errors.New("object")
		}
	case '[':
		for d.More() {
			if e = object(d, depth+1); e != nil {
				return e
			}
		}
		t, e = d.Token()
		if e != nil || t != json.Delim(']') {
			return errors.New("array")
		}
	default:
		return errors.New("delimiter")
	}
	return nil
}
