package dtos

import (
	"io"
	"strings"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type AccountChange struct{ Current, Value string }

func ParseAccountPassword(reader io.Reader) (AccountChange, error) {
	var p struct {
		Current  *string `json:"current_password"`
		Password *string `json:"password"`
	}
	o, err := Decode(reader, &p, "current_password", "password")
	if err != nil {
		return AccountChange{}, err
	}
	current, err := Text(o, "current_password", p.Current, 1, 128, false, true)
	if err != nil {
		return AccountChange{}, err
	}
	password, err := Text(o, "password", p.Password, 8, 128, false, true)
	if err != nil {
		return AccountChange{}, err
	}
	if !domain.StrongPassword(password) {
		return AccountChange{}, Invalid("password", "Password must include upper, lower, number, and symbol characters", "value_error")
	}
	return AccountChange{Current: current, Value: password}, nil
}
func ParseAccountEmail(reader io.Reader) (AccountChange, error) {
	var p struct {
		Current *string `json:"current_password"`
		Email   *string `json:"email"`
	}
	o, err := Decode(reader, &p, "current_password", "email")
	if err != nil {
		return AccountChange{}, err
	}
	current, err := Text(o, "current_password", p.Current, 1, 128, false, true)
	if err != nil {
		return AccountChange{}, err
	}
	address, err := Text(o, "email", p.Email, 5, 255, true, false)
	if err != nil {
		return AccountChange{}, err
	}
	if !email(address) {
		return AccountChange{}, Invalid("email", "A valid email address is required", "value_error")
	}
	address = strings.ToLower(address)
	return AccountChange{Current: current, Value: strings.ToLower(address)}, nil
}
