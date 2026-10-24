package dtos

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Decode excludes caller-supplied actor, source, revision and approval timestamps.
func GRNsDecode(r io.Reader) (domain.GRNsCommand, error) {
	var q domain.GRNsCommand
	data, e := io.ReadAll(io.LimitReader(r, 65537))
	if e != nil {
		return q, e
	}
	if len(data) > 65536 || !utf8.Valid(data) {
		return q, domain.GRNsInvalid("body", "Use a UTF-8 grn command within 64 KiB")
	}
	shape := json.NewDecoder(strings.NewReader(string(data)))
	shape.UseNumber()
	token, e := shape.Token()
	if e != nil || token != json.Delim('{') {
		return q, domain.GRNsInvalid("body", "Supply a site material grn command object")
	}
	if e = GRNsObject(shape, 0); e != nil {
		return q, e
	}
	if _, e = shape.Token(); e != io.EOF {
		return q, domain.GRNsInvalid("body", "Supply exactly one grn command")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		return q, domain.GRNsInvalid("body", "Supply only declared grn command fields")
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(data, &raw); e != nil {
		return q, e
	}
	for _, key := range []string{"version", "action", "reason"} {
		if _, ok := raw[key]; !ok {
			return q, domain.GRNsInvalid(key, "Supply the required command field")
		}
	}
	allowed := map[string][]string{"record": {"grn"}, "revise": {}}
	fields, ok := allowed[q.Action]
	if !ok {
		return q, domain.GRNsInvalid("action", "Choose record or revise")
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
			return q, domain.GRNsInvalid(key, "Field does not belong to this action")
		}
	}
	return domain.GRNsValidate(q)
}
func GRNsObject(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.GRNsInvalid("body", "GRN inwarding command nesting exceeds the limit")
	}
	keys := map[string]bool{}
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return domain.GRNsInvalid("body", "Invalid grn JSON")
		}
		key, ok := token.(string)
		if !ok || keys[key] {
			return domain.GRNsInvalid("body", "GRN inwarding JSON keys must be unique")
		}
		keys[key] = true
		value, e := d.Token()
		if e != nil || value == nil {
			return domain.GRNsInvalid("body", "Omit optional fields instead of null")
		}
		if delimiter, ok := value.(json.Delim); ok {
			switch delimiter {
			case '{':
				if e := GRNsObject(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e := GRNsArray(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.GRNsInvalid("body", "Invalid grn JSON nesting")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim('}') {
		return domain.GRNsInvalid("body", "Invalid grn JSON object")
	}
	return nil
}

func GRNsArray(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.GRNsInvalid("body", "GRN command nesting exceeds the limit")
	}
	count := 0
	for d.More() {
		count++
		if count > 100 {
			return domain.GRNsInvalid("body", "Keep JSON arrays within 100 entries")
		}
		v, e := d.Token()
		if e != nil || v == nil {
			return domain.GRNsInvalid("body", "Invalid or null checklist value")
		}
		if delim, ok := v.(json.Delim); ok {
			switch delim {
			case '{':
				if e = GRNsObject(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e = GRNsArray(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.GRNsInvalid("body", "Invalid grn array")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim(']') {
		return domain.GRNsInvalid("body", "Invalid grn array")
	}
	return nil
}
