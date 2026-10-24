package dtos

import (
	"encoding/json"
	"errors"
	"io"

	"time"
	"unicode/utf8"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ManufacturerResponse struct {
	ID        string         `json:"id"`
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Data      map[string]any `json:"data"`
	Active    bool           `json:"is_active"`
	Version   int32          `json:"version"`
	CreatedAt string         `json:"created_at"`
	CreatedBy *string        `json:"created_by_id"`
	UpdatedAt string         `json:"updated_at"`
	UpdatedBy *string        `json:"updated_by_id"`
}

func manufacturerStamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func ManufacturerFromDomain(v masterDomain.Manufacturer) ManufacturerResponse {
	return ManufacturerResponse{v.ID, v.Code, v.Name, v.Data, v.Active, v.Version, manufacturerStamp(v.CreatedAt), v.CreatedBy, manufacturerStamp(v.UpdatedAt), v.UpdatedBy}
}

type ManufacturerInput struct {
	Code, Name, Reason string
	Data               masterDomain.ManufacturerData
	Version            int32
	Active             bool
}

func manufacturerText(m map[string]json.RawMessage, k string, min, max int, required bool) (string, bool) {
	raw, exists := m[k]
	if !exists {
		return "", !required
	}
	var v string
	if string(raw) == "null" || UnmarshalMasterJSON(raw, &v) != nil {
		return "", false
	}
	v = masterDomain.ManufacturerTrim(v)
	n := utf8.RuneCountInString(v)
	return v, n >= min && n <= max
}

func manufacturerVersion(raw json.RawMessage) (int32, bool) {
	var value int32
	if UnmarshalMasterJSON(raw, &value) != nil || value < 1 {
		return 0, false
	}
	return value, true
}
func ParseManufacturer(body io.Reader, kind string) (ManufacturerInput, string, error) {
	var in ManufacturerInput
	raw, e := io.ReadAll(body)
	if e != nil {
		return in, "", e
	}
	allowed := "name data version reason"
	if kind == "create" {
		allowed = "code name data reason"
	}
	if kind == "active" {
		allowed = "is_active version reason"
	}
	m, e := StrictObject(raw, allowed)
	if e != nil {
		return in, "", errors.New("invalid manufacturer request")
	}
	var ok bool
	in.Reason, ok = manufacturerText(m, "reason", 1, 2000, true)
	if !ok {
		return in, "reason", errors.New("invalid manufacturer request")
	}
	if kind != "create" {
		in.Version, ok = manufacturerVersion(m["version"])
		if !ok {
			return in, "version", errors.New("invalid manufacturer request")
		}
	}
	if kind == "active" {
		in.Active, ok = JsonBoolean(m["is_active"])
		if !ok {
			return in, "is_active", errors.New("invalid manufacturer request")
		}
		return in, "", nil
	}
	in.Name, ok = manufacturerText(m, "name", 1, 255, true)
	if !ok {
		return in, "name", errors.New("invalid manufacturer request")
	}
	if kind == "create" {
		in.Code, ok = manufacturerText(m, "code", 1, 16, true)
		if !ok {
			return in, "code", errors.New("invalid manufacturer request")
		}
		clean, e := masterDomain.ManufacturerValidateCode(in.Code)
		if e != nil || clean != in.Code {
			return in, "code", errors.New("invalid manufacturer request")
		}
	}
	dm, e := StrictObject(m["data"], "eligible notes")
	if e != nil {
		return in, "data", errors.New("invalid manufacturer request")
	}
	in.Data.Notes, ok = manufacturerText(dm, "notes", 0, 2000, false)
	if !ok {
		return in, "data", errors.New("invalid manufacturer request")
	}
	var eligible []json.RawMessage
	if UnmarshalMasterJSON(dm["eligible"], &eligible) != nil || len(eligible) < 1 || len(eligible) > 200 {
		return in, "data", errors.New("invalid manufacturer request")
	}
	in.Data.Eligible = []string{}
	for _, v := range eligible {
		var code string
		if string(v) == "null" || UnmarshalMasterJSON(v, &code) != nil {
			return in, "data", errors.New("invalid manufacturer request")
		}
		in.Data.Eligible = append(in.Data.Eligible, masterDomain.ManufacturerTrim(code))
	}
	return in, "", nil
}

func ManufacturerHistoryFromDomain(v masterDomain.ManufacturerEvent) map[string]any {
	return map[string]any{"id": v.ID, "subject_id": v.SubjectID, "action": v.Action, "reason": v.Reason, "before": v.Before, "after": v.After, "created_at": manufacturerStamp(v.CreatedAt), "created_by_id": v.Actor}
}
