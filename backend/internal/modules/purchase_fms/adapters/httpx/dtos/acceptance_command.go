package dtos

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Decode excludes caller-supplied actor, source, revision and approval timestamps.
func AcceptanceDecode(r io.Reader) (domain.AcceptanceCommand, error) {
	var q domain.AcceptanceCommand
	data, e := io.ReadAll(io.LimitReader(r, 65537))
	if e != nil {
		return q, e
	}
	if len(data) > 65536 || !utf8.Valid(data) {
		return q, domain.AcceptanceInvalid("body", "Use a UTF-8 vendor-acceptance command within 64 KiB")
	}
	shape := json.NewDecoder(strings.NewReader(string(data)))
	shape.UseNumber()
	token, e := shape.Token()
	if e != nil || token != json.Delim('{') {
		return q, domain.AcceptanceInvalid("body", "Supply a vendor-acceptance command object")
	}
	if e = AcceptanceObject(shape, 0); e != nil {
		return q, e
	}
	if _, e = shape.Token(); e != io.EOF {
		return q, domain.AcceptanceInvalid("body", "Supply exactly one vendor-acceptance command")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		return q, domain.AcceptanceInvalid("body", "Supply only declared vendor-acceptance command fields")
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(data, &raw); e != nil {
		return q, e
	}
	for _, key := range []string{"version", "action", "reason", "response"} {
		if _, ok := raw[key]; !ok {
			return q, domain.AcceptanceInvalid(key, "Supply the required command field")
		}
	}
	allowed := map[string][]string{"record": {"response"}}
	fields, ok := allowed[q.Action]
	if !ok {
		return q, domain.AcceptanceInvalid("action", "Choose record")
	}
	for key := range raw {
		if key == "version" || key == "action" || key == "reason" {
			continue
		}
		found := false
		for _, field := range fields {
			found = found || field == key
		}
		if !found {
			return q, domain.AcceptanceInvalid(key, "Field does not belong to this action")
		}
	}
	return domain.AcceptanceValidate(q)
}
func AcceptanceObject(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.AcceptanceInvalid("body", "Vendor-acceptance command nesting exceeds the limit")
	}
	keys := map[string]bool{}
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return domain.AcceptanceInvalid("body", "Invalid vendor-acceptance JSON")
		}
		key, ok := token.(string)
		if !ok || keys[key] {
			return domain.AcceptanceInvalid("body", "Vendor-acceptance JSON keys must be unique")
		}
		keys[key] = true
		value, e := d.Token()
		if e != nil || value == nil {
			return domain.AcceptanceInvalid("body", "Omit optional fields instead of null")
		}
		if delimiter, ok := value.(json.Delim); ok {
			if delimiter != '{' {
				return domain.AcceptanceInvalid("body", "Use declared object and scalar fields")
			}
			if e := AcceptanceObject(d, depth+1); e != nil {
				return e
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim('}') {
		return domain.AcceptanceInvalid("body", "Invalid vendor-acceptance JSON object")
	}
	return nil
}
