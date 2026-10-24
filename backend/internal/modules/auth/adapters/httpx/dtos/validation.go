package dtos

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

type Issue struct {
	Type string   `json:"type"`
	Loc  []string `json:"loc"`
	Msg  string   `json:"msg"`
}

func (i *Issue) Error() string { return i.Msg }
func Invalid(field, message, kind string) *Issue {
	loc := []string{"body"}
	if field != "" {
		loc = append(loc, field)
	}
	return &Issue{Type: kind, Loc: loc, Msg: message}
}
func Decode(reader io.Reader, target any, allowed ...string) (map[string]json.RawMessage, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err = decoder.Decode(&object); err != nil || object == nil {
		return nil, Invalid("", "JSON object required", "json_invalid")
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return nil, Invalid("", "JSON decode error", "json_invalid")
	}
	for key := range object {
		known := false
		for _, field := range allowed {
			if key == field {
				known = true
			}
		}
		if !known {
			return nil, Invalid(key, "Extra inputs are not permitted", "extra_forbidden")
		}
	}
	// Pydantic-compatible nullable booleans; string/list fields stay typed.
	if active, present := object["is_active"]; present && string(active) != "null" {
		var value bool
		if err := json.Unmarshal(active, &value); err != nil {
			var candidate string
			if err = json.Unmarshal(active, &candidate); err != nil {
				candidate = string(active)
			}
			switch strings.ToLower(candidate) {
			case "true", "1", "yes", "on", "t", "y":
				value = true
			case "false", "0", "no", "off", "f", "n":
				value = false
			default:
				return nil, Invalid("is_active", "Input should be a valid boolean", "bool_parsing")
			}
		}
		object["is_active"], _ = json.Marshal(value)
	}
	for _, field := range []string{"role_names", "permission_codes"} {
		if value, present := object[field]; present && string(value) != "null" {
			var elements []json.RawMessage
			if err := json.Unmarshal(value, &elements); err != nil {
				return nil, Invalid(field, "Input should be a valid list", "list_type")
			}
			for _, element := range elements {
				var item string
				if string(element) == "null" || json.Unmarshal(element, &item) != nil {
					return nil, Invalid(field, "Input should be a valid string", "string_type")
				}
			}
		}
	}
	raw, _ = json.Marshal(object)
	typed := json.NewDecoder(bytes.NewReader(raw))
	typed.DisallowUnknownFields()
	if err = typed.Decode(target); err != nil {
		var wrong *json.UnmarshalTypeError
		field := ""
		if errors.As(err, &wrong) {
			field = wrong.Field
		}
		return nil, Invalid(field, "Invalid field type", "type_error")
	}
	return object, nil
}
func Text(object map[string]json.RawMessage, field string, value *string, min, max int, trim, required bool) (string, error) {
	if value == nil {
		if required {
			kind, msg := "missing", "Field required"
			if _, present := object[field]; present {
				kind, msg = "string_type", "Input should be a valid string"
			}
			return "", Invalid(field, msg, kind)
		}
		return "", nil
	}
	result := *value
	if trim {
		result = strings.TrimSpace(result)
	}
	n := utf8.RuneCountInString(result)
	if n < min || n > max {
		return "", Invalid(field, "String length is outside the permitted range", "string_length")
	}
	return result, nil
}
func List(field string, values *[]string, min, max int, required bool) error {
	if values == nil {
		if required {
			return Invalid(field, "Field required", "missing")
		}
		return nil
	}
	if len(*values) < min || len(*values) > max {
		return Invalid(field, "List length is outside the permitted range", "list_length")
	}
	for i := range *values {
		(*values)[i] = strings.TrimSpace((*values)[i])
	}
	return nil
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
var rolePattern = regexp.MustCompile(`^[a-z0-9._-]+$`)
var activityPattern = regexp.MustCompile(`^/[a-zA-Z0-9_./?=&%\-]*$`)

func email(value string) bool {
	return strings.Contains(value, "@") && !strings.HasPrefix(value, "@") && !strings.HasSuffix(value, "@")
}
