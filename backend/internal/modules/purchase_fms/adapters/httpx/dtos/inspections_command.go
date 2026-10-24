package dtos

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Decode excludes caller-supplied actor, source, revision and approval timestamps.
func InspectionsDecode(r io.Reader) (domain.InspectionsCommand, error) {
	var q domain.InspectionsCommand
	data, e := io.ReadAll(io.LimitReader(r, 65537))
	if e != nil {
		return q, e
	}
	if len(data) > 65536 || !utf8.Valid(data) {
		return q, domain.InspectionsInvalid("body", "Use a UTF-8 inspection command within 64 KiB")
	}
	shape := json.NewDecoder(strings.NewReader(string(data)))
	shape.UseNumber()
	token, e := shape.Token()
	if e != nil || token != json.Delim('{') {
		return q, domain.InspectionsInvalid("body", "Supply a inspection command object")
	}
	if e = InspectionsObject(shape, 0); e != nil {
		return q, e
	}
	if _, e = shape.Token(); e != io.EOF {
		return q, domain.InspectionsInvalid("body", "Supply exactly one inspection command")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		return q, domain.InspectionsInvalid("body", "Supply only declared inspection command fields")
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(data, &raw); e != nil {
		return q, e
	}
	for _, key := range []string{"version", "action", "reason"} {
		if _, ok := raw[key]; !ok {
			return q, domain.InspectionsInvalid(key, "Supply the required command field")
		}
	}
	allowed := map[string][]string{"record": {"inspection"}, "revise": {}}
	fields, ok := allowed[q.Action]
	if !ok {
		return q, domain.InspectionsInvalid("action", "Choose record or revise")
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
			return q, domain.InspectionsInvalid(key, "Field does not belong to this action")
		}
	}
	return domain.InspectionsValidate(q)
}
func InspectionsObject(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.InspectionsInvalid("body", "Vendor-acceptance command nesting exceeds the limit")
	}
	keys := map[string]bool{}
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return domain.InspectionsInvalid("body", "Invalid inspection JSON")
		}
		key, ok := token.(string)
		if !ok || keys[key] {
			return domain.InspectionsInvalid("body", "Vendor-acceptance JSON keys must be unique")
		}
		keys[key] = true
		value, e := d.Token()
		if e != nil || value == nil {
			return domain.InspectionsInvalid("body", "Omit optional fields instead of null")
		}
		if delimiter, ok := value.(json.Delim); ok {
			switch delimiter {
			case '{':
				if e := InspectionsObject(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e := InspectionsArray(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.InspectionsInvalid("body", "Invalid inspection JSON nesting")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim('}') {
		return domain.InspectionsInvalid("body", "Invalid inspection JSON object")
	}
	return nil
}

func InspectionsArray(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.InspectionsInvalid("body", "Inspection command nesting exceeds the limit")
	}
	count := 0
	for d.More() {
		count++
		if count > 100 {
			return domain.InspectionsInvalid("witness_points", "Keep the inspection checklist within 100 points")
		}
		v, e := d.Token()
		if e != nil || v == nil {
			return domain.InspectionsInvalid("body", "Invalid or null checklist value")
		}
		if delim, ok := v.(json.Delim); ok {
			switch delim {
			case '{':
				if e = InspectionsObject(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e = InspectionsArray(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.InspectionsInvalid("body", "Invalid inspection array")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim(']') {
		return domain.InspectionsInvalid("body", "Invalid inspection array")
	}
	return nil
}
