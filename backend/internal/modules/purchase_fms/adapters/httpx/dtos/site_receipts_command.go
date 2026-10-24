package dtos

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Decode excludes caller-supplied actor, source, revision and approval timestamps.
func SiteReceiptsDecode(r io.Reader) (domain.SiteReceiptsCommand, error) {
	var q domain.SiteReceiptsCommand
	data, e := io.ReadAll(io.LimitReader(r, 65537))
	if e != nil {
		return q, e
	}
	if len(data) > 65536 || !utf8.Valid(data) {
		return q, domain.SiteReceiptsInvalid("body", "Use a UTF-8 receipt command within 64 KiB")
	}
	shape := json.NewDecoder(strings.NewReader(string(data)))
	shape.UseNumber()
	token, e := shape.Token()
	if e != nil || token != json.Delim('{') {
		return q, domain.SiteReceiptsInvalid("body", "Supply a site material receipt command object")
	}
	if e = SiteReceiptsObject(shape, 0); e != nil {
		return q, e
	}
	if _, e = shape.Token(); e != io.EOF {
		return q, domain.SiteReceiptsInvalid("body", "Supply exactly one receipt command")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		return q, domain.SiteReceiptsInvalid("body", "Supply only declared receipt command fields")
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(data, &raw); e != nil {
		return q, e
	}
	for _, key := range []string{"version", "action", "reason"} {
		if _, ok := raw[key]; !ok {
			return q, domain.SiteReceiptsInvalid(key, "Supply the required command field")
		}
	}
	allowed := map[string][]string{"record": {"receipt"}, "revise": {}}
	fields, ok := allowed[q.Action]
	if !ok {
		return q, domain.SiteReceiptsInvalid("action", "Choose record or revise")
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
			return q, domain.SiteReceiptsInvalid(key, "Field does not belong to this action")
		}
	}
	return domain.SiteReceiptsValidate(q)
}
func SiteReceiptsObject(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.SiteReceiptsInvalid("body", "Site material receipt command nesting exceeds the limit")
	}
	keys := map[string]bool{}
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return domain.SiteReceiptsInvalid("body", "Invalid receipt JSON")
		}
		key, ok := token.(string)
		if !ok || keys[key] {
			return domain.SiteReceiptsInvalid("body", "Site material receipt JSON keys must be unique")
		}
		keys[key] = true
		value, e := d.Token()
		if e != nil || value == nil {
			return domain.SiteReceiptsInvalid("body", "Omit optional fields instead of null")
		}
		if delimiter, ok := value.(json.Delim); ok {
			switch delimiter {
			case '{':
				if e := SiteReceiptsObject(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e := SiteReceiptsArray(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.SiteReceiptsInvalid("body", "Invalid receipt JSON nesting")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim('}') {
		return domain.SiteReceiptsInvalid("body", "Invalid receipt JSON object")
	}
	return nil
}

func SiteReceiptsArray(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.SiteReceiptsInvalid("body", "Receipt command nesting exceeds the limit")
	}
	count := 0
	for d.More() {
		count++
		if count > 100 {
			return domain.SiteReceiptsInvalid("body", "Keep JSON arrays within 100 entries")
		}
		v, e := d.Token()
		if e != nil || v == nil {
			return domain.SiteReceiptsInvalid("body", "Invalid or null checklist value")
		}
		if delim, ok := v.(json.Delim); ok {
			switch delim {
			case '{':
				if e = SiteReceiptsObject(d, depth+1); e != nil {
					return e
				}
			case '[':
				if e = SiteReceiptsArray(d, depth+1); e != nil {
					return e
				}
			default:
				return domain.SiteReceiptsInvalid("body", "Invalid receipt array")
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim(']') {
		return domain.SiteReceiptsInvalid("body", "Invalid receipt array")
	}
	return nil
}
