package dtos

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Decode excludes caller-supplied actor, source, revision and approval timestamps.
func EngineerConfirmationsDecode(r io.Reader) (domain.EngineerConfirmationsCommand, error) {
	var q domain.EngineerConfirmationsCommand
	data, e := io.ReadAll(io.LimitReader(r, 65537))
	if e != nil {
		return q, e
	}
	if len(data) > 65536 || !utf8.Valid(data) {
		return q, domain.EngineerConfirmationsInvalid("body", "Use a UTF-8 confirmation command within 64 KiB")
	}
	shape := json.NewDecoder(strings.NewReader(string(data)))
	shape.UseNumber()
	token, e := shape.Token()
	if e != nil || token != json.Delim('{') {
		return q, domain.EngineerConfirmationsInvalid("body", "Supply a site material confirmation command object")
	}
	if e = EngineerConfirmationsObject(shape, 0); e != nil {
		return q, e
	}
	if _, e = shape.Token(); e != io.EOF {
		return q, domain.EngineerConfirmationsInvalid("body", "Supply exactly one confirmation command")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		return q, domain.EngineerConfirmationsInvalid("body", "Supply only declared confirmation command fields")
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(data, &raw); e != nil {
		return q, e
	}
	for _, key := range []string{"version", "action", "reason"} {
		if _, ok := raw[key]; !ok {
			return q, domain.EngineerConfirmationsInvalid(key, "Supply the required command field")
		}
	}
	allowed := map[string][]string{"record": {"confirmation"}, "revise": {}}
	fields, ok := allowed[q.Action]
	if !ok {
		return q, domain.EngineerConfirmationsInvalid("action", "Choose record or revise")
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
			return q, domain.EngineerConfirmationsInvalid(key, "Field does not belong to this action")
		}
	}
	return domain.EngineerConfirmationsValidate(q)
}
func EngineerConfirmationsObject(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.EngineerConfirmationsInvalid("body", "Engineer confirmation command nesting exceeds the limit")
	}
	keys := map[string]bool{}
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return domain.EngineerConfirmationsInvalid("body", "Invalid confirmation JSON")
		}
		key, ok := token.(string)
		if !ok || keys[key] {
			return domain.EngineerConfirmationsInvalid("body", "Engineer confirmation JSON keys must be unique")
		}
		keys[key] = true
		value, e := d.Token()
		if e != nil || value == nil {
			return domain.EngineerConfirmationsInvalid("body", "Omit optional fields instead of null")
		}
		if delimiter, ok := value.(json.Delim); ok {
			switch delimiter {
			case '{':
				if e := EngineerConfirmationsObject(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e := EngineerConfirmationsArray(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.EngineerConfirmationsInvalid("body", "Invalid confirmation JSON nesting")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim('}') {
		return domain.EngineerConfirmationsInvalid("body", "Invalid confirmation JSON object")
	}
	return nil
}

func EngineerConfirmationsArray(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.EngineerConfirmationsInvalid("body", "Confirmation command nesting exceeds the limit")
	}
	count := 0
	for d.More() {
		count++
		if count > 100 {
			return domain.EngineerConfirmationsInvalid("body", "Keep JSON arrays within 100 entries")
		}
		v, e := d.Token()
		if e != nil || v == nil {
			return domain.EngineerConfirmationsInvalid("body", "Invalid or null checklist value")
		}
		if delim, ok := v.(json.Delim); ok {
			switch delim {
			case '{':
				if e = EngineerConfirmationsObject(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e = EngineerConfirmationsArray(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.EngineerConfirmationsInvalid("body", "Invalid confirmation array")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim(']') {
		return domain.EngineerConfirmationsInvalid("body", "Invalid confirmation array")
	}
	return nil
}
