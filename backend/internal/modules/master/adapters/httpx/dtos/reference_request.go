package dtos

import (
	"encoding/json"
	"errors"
	"io"

	"strings"
	"unicode/utf8"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ReferenceResponse struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Code   string  `json:"code"`
	Active bool    `json:"is_active"`
	State  *string `json:"state_id"`
}

func ReferenceFromDomain(v masterDomain.Reference) ReferenceResponse {
	var state *string
	if v.StateID != "" {
		state = &v.StateID
	}
	return ReferenceResponse{v.ID, v.Name, v.Code, v.Active, state}
}

func ReferenceBoolean(s string) (bool, bool) {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "y", "on", "t":
		return true, true
	case "0", "false", "no", "n", "off", "f":
		return false, true
	}
	return false, false
}

func ParseReference(body io.Reader) (masterDomain.ReferenceInput, string, error) {
	var in masterDomain.ReferenceInput
	data, e := io.ReadAll(body)
	if e != nil {
		return in, "", e
	}
	var m map[string]json.RawMessage
	if e = UnmarshalMasterJSON(data, &m); e != nil || m == nil {
		return in, "", errors.New("object required")
	}
	for k := range m {
		if k != "name" && k != "code" && k != "state_id" {
			return in, "", errors.New("unknown field")
		}
	}
	for _, f := range []struct {
		name     string
		dest     *string
		min, max int
	}{{"name", &in.Name, 2, 128}, {"code", &in.Code, 0, 32}} {
		v := ""
		raw, exists := m[f.name]
		if !exists && f.min > 0 {
			return in, f.name, errors.New("required")
		}
		if exists && (string(raw) == "null" || UnmarshalMasterJSON(raw, &v) != nil) {
			return in, f.name, errors.New("string required")
		}
		v = masterDomain.ReferenceTrim(v)
		n := utf8.RuneCountInString(v)
		if n < f.min || n > f.max {
			return in, f.name, errors.New("length")
		}
		*f.dest = v
	}
	if raw, ok := m["state_id"]; ok && string(raw) != "null" {
		var v string
		if UnmarshalMasterJSON(raw, &v) != nil {
			return in, "state_id", errors.New("UUID")
		}
		in.StateID, e = masterDomain.ReferenceCanonicalID(v)
		if e != nil {
			return in, "state_id", e
		}
	}
	return in, "", nil
}
