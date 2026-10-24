package inbound

import (
	"encoding/json"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
	"io"
	"strings"
	"unicode/utf8"
)

// Decode excludes caller-supplied actor, source, revision and approval timestamps.
func Decode(r io.Reader) (domain.Command, error) {
	var q domain.Command
	data, e := io.ReadAll(io.LimitReader(r, 65537))
	if e != nil {
		return q, e
	}
	if len(data) > 65536 || !utf8.Valid(data) {
		return q, domain.Invalid("body", "Use a UTF-8 acceptance command within 64 KiB")
	}
	shape := json.NewDecoder(strings.NewReader(string(data)))
	shape.UseNumber()
	token, e := shape.Token()
	if e != nil || token != json.Delim('{') {
		return q, domain.Invalid("body", "Supply a site material acceptance command object")
	}
	if e = object(shape, 0); e != nil {
		return q, e
	}
	if _, e = shape.Token(); e != io.EOF {
		return q, domain.Invalid("body", "Supply exactly one acceptance command")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		return q, domain.Invalid("body", "Supply only declared acceptance command fields")
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(data, &raw); e != nil {
		return q, e
	}
	for _, key := range []string{"version", "action", "reason"} {
		if _, ok := raw[key]; !ok {
			return q, domain.Invalid(key, "Supply the required command field")
		}
	}
	allowed := map[string][]string{"record": {"acceptance"}, "revise": {}}
	fields, ok := allowed[q.Action]
	if !ok {
		return q, domain.Invalid("action", "Choose record or revise")
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
			return q, domain.Invalid(key, "Field does not belong to this action")
		}
	}
	return domain.Validate(q)
}
func object(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.Invalid("body", "Accounts acceptance command nesting exceeds the limit")
	}
	keys := map[string]bool{}
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return domain.Invalid("body", "Invalid acceptance JSON")
		}
		key, ok := token.(string)
		if !ok || keys[key] {
			return domain.Invalid("body", "Accounts acceptance JSON keys must be unique")
		}
		keys[key] = true
		value, e := d.Token()
		if e != nil || value == nil {
			return domain.Invalid("body", "Omit optional fields instead of null")
		}
		if delimiter, ok := value.(json.Delim); ok {
			switch delimiter {
			case '{':
				if e := object(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e := array(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.Invalid("body", "Invalid acceptance JSON nesting")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim('}') {
		return domain.Invalid("body", "Invalid acceptance JSON object")
	}
	return nil
}

func array(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.Invalid("body", "Acceptance command nesting exceeds the limit")
	}
	count := 0
	for d.More() {
		count++
		if count > 100 {
			return domain.Invalid("body", "Keep JSON arrays within 100 entries")
		}
		v, e := d.Token()
		if e != nil || v == nil {
			return domain.Invalid("body", "Invalid or null checklist value")
		}
		if delim, ok := v.(json.Delim); ok {
			switch delim {
			case '{':
				if e = object(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e = array(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.Invalid("body", "Invalid acceptance array")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim(']') {
		return domain.Invalid("body", "Invalid acceptance array")
	}
	return nil
}
