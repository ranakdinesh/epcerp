package dtos

import (
	"io"
	"strings"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type RecoveryRequest struct{ Email string }
type RecoveryComplete struct{ Token, Password string }
type RecoveryMessage struct {
	Message string `json:"message"`
}

func ParseRecoveryRequest(reader io.Reader) (RecoveryRequest, error) {
	var p struct {
		Email *string `json:"email"`
	}
	o, err := Decode(reader, &p, "email")
	if err != nil {
		return RecoveryRequest{}, err
	}
	value, err := Text(o, "email", p.Email, 5, 255, true, false)
	if err != nil {
		return RecoveryRequest{}, err
	}
	if !email(value) {
		return RecoveryRequest{}, Invalid("email", "Enter a valid registered email address", "value_error")
	}
	return RecoveryRequest{strings.ToLower(value)}, nil
}
func ParseRecoveryComplete(reader io.Reader) (RecoveryComplete, error) {
	var p struct {
		Token    *string `json:"token"`
		Password *string `json:"password"`
	}
	o, err := Decode(reader, &p, "token", "password")
	if err != nil {
		return RecoveryComplete{}, err
	}
	token, err := Text(o, "token", p.Token, 64, 64, false, true)
	if err != nil {
		return RecoveryComplete{}, err
	}
	password, err := Text(o, "password", p.Password, 8, 128, false, true)
	if err != nil {
		return RecoveryComplete{}, err
	}
	if !domain.StrongPassword(password) {
		return RecoveryComplete{}, Invalid("password", "Use 8 to 128 characters including uppercase, lowercase, number and symbol", "value_error")
	}
	return RecoveryComplete{token, password}, nil
}
