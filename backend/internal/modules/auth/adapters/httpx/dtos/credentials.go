package dtos

import (
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"io"
)

func ParseResetReason(reader io.Reader) (string, error) {
	var p struct {
		Reason *string `json:"reason"`
	}
	o, e := Decode(reader, &p, "reason")
	if e != nil {
		return "", e
	}
	s, e := Text(o, "reason", p.Reason, 10, 1000, true, true)
	if e != nil {
		return "", e
	}
	if !domain.ValidResetReason(s) {
		return "", Invalid("reason", "Provide a reset reason of 10 to 1000 characters", "value_error")
	}
	return s, nil
}
