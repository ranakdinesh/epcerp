package dtos

import (
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"io"
)

type UsernameChange struct {
	Username *string `json:"username"`
	Expected *string `json:"expected_username"`
}

func ParseUsernameChange(reader io.Reader) (UsernameChange, error) {
	var p UsernameChange
	o, e := Decode(reader, &p, "username", "expected_username")
	if e != nil {
		return p, e
	}
	name, e := Text(o, "username", p.Username, 3, 64, true, true)
	if e != nil {
		return p, e
	}
	name, e = domain.Username(name)
	if e != nil {
		return p, Invalid("username", e.Error(), "value_error")
	}
	p.Username = &name
	_, e = Text(o, "expected_username", p.Expected, 3, 64, false, true)
	return p, e
}

type UsernameAvailability struct {
	Available bool `json:"available"`
}
