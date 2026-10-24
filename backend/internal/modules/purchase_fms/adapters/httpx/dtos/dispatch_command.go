package dtos

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Decode excludes caller-supplied actor, source, revision and approval timestamps.
func DispatchDecode(r io.Reader) (domain.DispatchCommand, error) {
	var q domain.DispatchCommand
	data, e := io.ReadAll(io.LimitReader(r, 65537))
	if e != nil {
		return q, e
	}
	if len(data) > 65536 || !utf8.Valid(data) {
		return q, domain.DispatchInvalid("body", "Use a UTF-8 dispatch command within 64 KiB")
	}
	shape := json.NewDecoder(strings.NewReader(string(data)))
	shape.UseNumber()
	token, e := shape.Token()
	if e != nil || token != json.Delim('{') {
		return q, domain.DispatchInvalid("body", "Supply an Material dispatch command object")
	}
	if e = DispatchObject(shape, 0); e != nil {
		return q, e
	}
	if _, e = shape.Token(); e != io.EOF {
		return q, domain.DispatchInvalid("body", "Supply exactly one dispatch command")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		return q, domain.DispatchInvalid("body", "Supply only declared dispatch command fields")
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(data, &raw); e != nil {
		return q, e
	}
	for _, key := range []string{"version", "action", "reason"} {
		if _, ok := raw[key]; !ok {
			return q, domain.DispatchInvalid(key, "Supply the required command field")
		}
	}
	allowed := map[string][]string{"record": {"dispatch"}, "revise": {}}
	fields, ok := allowed[q.Action]
	if !ok {
		return q, domain.DispatchInvalid("action", "Choose record or revise")
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
			return q, domain.DispatchInvalid(key, "Field does not belong to this action")
		}
	}
	return domain.DispatchValidate(q)
}
func DispatchObject(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.DispatchInvalid("body", "Material dispatch command nesting exceeds the limit")
	}
	keys := map[string]bool{}
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return domain.DispatchInvalid("body", "Invalid dispatch JSON")
		}
		key, ok := token.(string)
		if !ok || keys[key] {
			return domain.DispatchInvalid("body", "Material dispatch JSON keys must be unique")
		}
		keys[key] = true
		value, e := d.Token()
		if e != nil || value == nil {
			return domain.DispatchInvalid("body", "Omit optional fields instead of null")
		}
		if delimiter, ok := value.(json.Delim); ok {
			switch delimiter {
			case '{':
				if e := DispatchObject(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e := DispatchArray(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.DispatchInvalid("body", "Invalid dispatch JSON nesting")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim('}') {
		return domain.DispatchInvalid("body", "Invalid dispatch JSON object")
	}
	return nil
}

func DispatchArray(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.DispatchInvalid("body", "Dispatch command nesting exceeds the limit")
	}
	count := 0
	for d.More() {
		count++
		if count > 100 {
			return domain.DispatchInvalid("witness_points", "Keep the dispatch checklist within 100 points")
		}
		v, e := d.Token()
		if e != nil || v == nil {
			return domain.DispatchInvalid("body", "Invalid or null checklist value")
		}
		if delim, ok := v.(json.Delim); ok {
			switch delim {
			case '{':
				if e = DispatchObject(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e = DispatchArray(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.DispatchInvalid("body", "Invalid dispatch array")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim(']') {
		return domain.DispatchInvalid("body", "Invalid dispatch array")
	}
	return nil
}
