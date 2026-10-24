package dtos

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Decode excludes caller-supplied actor, source, revision and approval timestamps.
func OrdersDecode(r io.Reader) (domain.OrdersCommand, error) {
	var q domain.OrdersCommand
	data, e := io.ReadAll(io.LimitReader(r, 65537))
	if e != nil {
		return q, e
	}
	if len(data) > 65536 || !utf8.Valid(data) {
		return q, domain.OrdersInvalid("body", "Use a UTF-8 purchase-order command within 64 KiB")
	}
	shape := json.NewDecoder(strings.NewReader(string(data)))
	shape.UseNumber()
	token, e := shape.Token()
	if e != nil || token != json.Delim('{') {
		return q, domain.OrdersInvalid("body", "Supply a purchase-order command object")
	}
	if e = OrdersObject(shape, 0); e != nil {
		return q, e
	}
	if _, e = shape.Token(); e != io.EOF {
		return q, domain.OrdersInvalid("body", "Supply exactly one purchase-order command")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&q); e != nil {
		return q, domain.OrdersInvalid("body", "Supply only declared purchase-order command fields")
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal(data, &raw); e != nil {
		return q, e
	}
	for _, key := range []string{"version", "action", "reason"} {
		if _, ok := raw[key]; !ok {
			return q, domain.OrdersInvalid(key, "Supply the required command field")
		}
	}
	allowed := map[string][]string{"prepare": {"details"}, "release": {}, "submit_client": {"submission"}, "client_decision": {"client_decision"}}
	fields, ok := allowed[q.Action]
	if !ok {
		return q, domain.OrdersInvalid("action", "Choose prepare, submit_client, client_decision or release")
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
			return q, domain.OrdersInvalid(key, "Field does not belong to this action")
		}
	}
	return domain.OrdersValidate(q)
}
func OrdersObject(d *json.Decoder, depth int) error {
	if depth > 8 {
		return domain.OrdersInvalid("body", "Purchase-order command nesting exceeds the limit")
	}
	keys := map[string]bool{}
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return domain.OrdersInvalid("body", "Invalid purchase-order JSON")
		}
		key, ok := token.(string)
		if !ok || keys[key] {
			return domain.OrdersInvalid("body", "Purchase-order JSON keys must be unique")
		}
		keys[key] = true
		value, e := d.Token()
		if e != nil || value == nil {
			return domain.OrdersInvalid("body", "Omit optional fields instead of null")
		}
		if delimiter, ok := value.(json.Delim); ok {
			if delimiter != '{' {
				return domain.OrdersInvalid("body", "Use declared object and scalar fields")
			}
			if e := OrdersObject(d, depth+1); e != nil {
				return e
			}
		}
	}
	token, e := d.Token()
	if e != nil || token != json.Delim('}') {
		return domain.OrdersInvalid("body", "Invalid purchase-order JSON object")
	}
	return nil
}
