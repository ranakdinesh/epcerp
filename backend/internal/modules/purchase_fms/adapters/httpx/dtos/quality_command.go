package dtos

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Decode excludes caller-supplied actor, source, revision and approval timestamps.
func QualityDecode(r io.Reader) (domain.QualityCommand, error) {
	var q domain.QualityCommand
	data, e := io.ReadAll(io.LimitReader(r, 65537))
	if e != nil {
		return q, e
	}
	if len(data) > 65536 || !utf8.Valid(data) {
		return q, domain.QualityInvalid("body", "Use a UTF-8 quality command within 64 KiB")
	}
	shape := json.NewDecoder(strings.NewReader(string(data)))
	shape.UseNumber()
	token, e := shape.Token()
	if e != nil || token != json.Delim('{') {
		return q, domain.QualityInvalid("body", "Supply an Quality certificate command object")
	}
	if e = QualityObject(shape, 0); e != nil {
		return q, e
	}
	if _, e = shape.Token(); e != io.EOF {
		return q, domain.QualityInvalid("body", "Supply exactly one quality command")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		return q, domain.QualityInvalid("body", "Supply only declared quality command fields")
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(data, &raw); e != nil {
		return q, e
	}
	for _, key := range []string{"version", "action", "reason"} {
		if _, ok := raw[key]; !ok {
			return q, domain.QualityInvalid(key, "Supply the required command field")
		}
	}
	allowed := map[string][]string{"record": {"certificate_code", "details"}, "verify": {"certificate_code"}, "revoke": {"certificate_code"}, "revise": {}}
	fields, ok := allowed[q.Action]
	if !ok {
		return q, domain.QualityInvalid("action", "Choose record, verify, revoke or revise")
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
			return q, domain.QualityInvalid(key, "Field does not belong to this action")
		}
	}
	return domain.QualityValidate(q)
}
func QualityObject(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.QualityInvalid("body", "Quality certificate command nesting exceeds the limit")
	}
	keys := map[string]bool{}
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return domain.QualityInvalid("body", "Invalid quality JSON")
		}
		key, ok := token.(string)
		if !ok || keys[key] {
			return domain.QualityInvalid("body", "Quality certificate JSON keys must be unique")
		}
		keys[key] = true
		value, e := d.Token()
		if e != nil || value == nil {
			return domain.QualityInvalid("body", "Omit optional fields instead of null")
		}
		if delimiter, ok := value.(json.Delim); ok {
			switch delimiter {
			case '{':
				if e := QualityObject(d, depth+1); e != nil {
					return e
				}
			case '[':
				return domain.QualityInvalid("body", "Quality commands do not contain arrays")
			default:
				return domain.QualityInvalid("body", "Invalid quality JSON nesting")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim('}') {
		return domain.QualityInvalid("body", "Invalid quality JSON object")
	}
	return nil
}
