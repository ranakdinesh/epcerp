package inbound

import (
	"encoding/json"
	"errors"
	"regexp"
)

// Request translation keeps decimal lexemes exact; the owning domain checks
// monetary precision and quantity limits without passing through float64.
type Decimal string

var decimal = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

func (d *Decimal) UnmarshalJSON(raw []byte) error {
	if len(raw) > 64 {
		return errors.New("decimal too long")
	}
	s := string(raw)
	if len(s) > 0 && s[0] == '"' {
		if e := json.Unmarshal(raw, &s); e != nil {
			return e
		}
	}
	if !decimal.MatchString(s) {
		return errors.New("use a finite nonnegative plain decimal")
	}
	*d = Decimal(s)
	return nil
}
