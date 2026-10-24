package dtos

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
)

// Decimal retains the caller's exact decimal lexeme. JSON numbers never pass
// through float64; field precision and bounds belong to the owning domain.
type QuotationsDecimal string

var QuotationsPlainDecimal = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

func (v *QuotationsDecimal) UnmarshalJSON(data []byte) error {
	if len(data) > 64 {
		return errors.New("decimal is too long")
	}
	var s string
	if bytes.HasPrefix(data, []byte(`"`)) {
		if e := json.Unmarshal(data, &s); e != nil {
			return e
		}
	} else {
		s = string(data)
	}
	if !QuotationsPlainDecimal.MatchString(s) {
		return errors.New("use a finite nonnegative plain decimal")
	}
	*v = QuotationsDecimal(s)
	return nil
}
