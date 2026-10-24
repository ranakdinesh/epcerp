package dtos

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Decode excludes caller-supplied actor, source, revision and approval timestamps.
func TechnicalDecode(r io.Reader) (domain.TechnicalCommand, error) {
	var q domain.TechnicalCommand
	data, e := io.ReadAll(io.LimitReader(r, 65537))
	if e != nil {
		return q, e
	}
	if len(data) > 65536 || !utf8.Valid(data) {
		return q, domain.TechnicalInvalid("body", "Use a UTF-8 technical command within 64 KiB")
	}
	shape := json.NewDecoder(strings.NewReader(string(data)))
	shape.UseNumber()
	token, e := shape.Token()
	if e != nil || token != json.Delim('{') {
		return q, domain.TechnicalInvalid("body", "Supply a technical command object")
	}
	if e = TechnicalObject(shape, 0); e != nil {
		return q, e
	}
	if _, e = shape.Token(); e != io.EOF {
		return q, domain.TechnicalInvalid("body", "Supply exactly one technical command")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		return q, domain.TechnicalInvalid("body", "Supply only declared technical command fields")
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(data, &raw); e != nil {
		return q, e
	}
	for _, key := range []string{"version", "action", "reason"} {
		if _, ok := raw[key]; !ok {
			return q, domain.TechnicalInvalid(key, "Supply the required command field")
		}
	}
	allowed := map[string][]string{"configure": {}, "document": {"document"}, "submit": {"submission"}, "team_review": {"approve", "remarks"}, "client_approval": {"approval", "qap_approved", "conditions_closed"}, "revise": {}}
	fields, ok := allowed[q.Action]
	if !ok {
		return q, domain.TechnicalInvalid("action", "Choose a technical action")
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
			return q, domain.TechnicalInvalid(key, "Field does not belong to this action")
		}
	}
	return domain.TechnicalValidateCommand(q)
}
func TechnicalObject(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.TechnicalInvalid("body", "Technical command nesting exceeds the limit")
	}
	keys := map[string]bool{}
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return domain.TechnicalInvalid("body", "Invalid technical JSON")
		}
		key, ok := token.(string)
		if !ok || keys[key] {
			return domain.TechnicalInvalid("body", "Technical JSON keys must be unique")
		}
		keys[key] = true
		value, e := d.Token()
		if e != nil || value == nil {
			return domain.TechnicalInvalid("body", "Omit optional fields instead of null")
		}
		if delimiter, ok := value.(json.Delim); ok {
			if delimiter != '{' {
				return domain.TechnicalInvalid("body", "Use declared object and scalar fields")
			}
			if e := TechnicalObject(d, depth+1); e != nil {
				return e
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim('}') {
		return domain.TechnicalInvalid("body", "Invalid technical JSON object")
	}
	return nil
}
