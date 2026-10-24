package dtos

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Decode excludes caller-supplied actor, source, revision and approval timestamps.
func AdvancesDecode(r io.Reader) (domain.AdvancesCommand, error) {
	var q domain.AdvancesCommand
	data, e := io.ReadAll(io.LimitReader(r, 65537))
	if e != nil {
		return q, e
	}
	if len(data) > 65536 || !utf8.Valid(data) {
		return q, domain.AdvancesInvalid("body", "Use a UTF-8 advance-request command within 64 KiB")
	}
	shape := json.NewDecoder(strings.NewReader(string(data)))
	shape.UseNumber()
	token, e := shape.Token()
	if e != nil || token != json.Delim('{') {
		return q, domain.AdvancesInvalid("body", "Supply a advance-request command object")
	}
	if e = AdvancesObject(shape, 0); e != nil {
		return q, e
	}
	if _, e = shape.Token(); e != io.EOF {
		return q, domain.AdvancesInvalid("body", "Supply exactly one advance-request command")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		return q, domain.AdvancesInvalid("body", "Supply only declared advance-request command fields")
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(data, &raw); e != nil {
		return q, e
	}
	for _, key := range []string{"version", "action", "reason"} {
		if _, ok := raw[key]; !ok {
			return q, domain.AdvancesInvalid(key, "Supply the required command field")
		}
	}
	allowed := map[string][]string{"prepare": {"details"}, "submit": {}, "revise": {}, "approve": {}, "reject": {}, "disburse": {"payment"}}
	fields, ok := allowed[q.Action]
	if !ok {
		return q, domain.AdvancesInvalid("action", "Choose a declared advance action")
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
			return q, domain.AdvancesInvalid(key, "Field does not belong to this action")
		}
	}
	return domain.AdvancesValidate(q)
}
func AdvancesObject(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.AdvancesInvalid("body", "Advance-request command nesting exceeds the limit")
	}
	keys := map[string]bool{}
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return domain.AdvancesInvalid("body", "Invalid advance-request JSON")
		}
		key, ok := token.(string)
		if !ok || keys[key] {
			return domain.AdvancesInvalid("body", "Advance-request JSON keys must be unique")
		}
		keys[key] = true
		value, e := d.Token()
		if e != nil || value == nil {
			return domain.AdvancesInvalid("body", "Omit optional fields instead of null")
		}
		if delimiter, ok := value.(json.Delim); ok {
			if delimiter != '{' {
				return domain.AdvancesInvalid("body", "Use declared object and scalar fields")
			}
			if e := AdvancesObject(d, depth+1); e != nil {
				return e
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim('}') {
		return domain.AdvancesInvalid("body", "Invalid advance-request JSON object")
	}
	return nil
}
