package dtos

import (
	"io"
	"strings"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type Credentials struct{ Identifier, Password string }

func ParseLogin(reader io.Reader) (Credentials, error) {
	var payload struct {
		Identifier *string `json:"identifier"`
		Password   *string `json:"password"`
	}
	object, err := Decode(reader, &payload, "identifier", "password")
	if err != nil {
		return Credentials{}, err
	}
	identifier, err := Text(object, "identifier", payload.Identifier, 3, 255, true, true)
	if err != nil {
		return Credentials{}, err
	}
	password, err := Text(object, "password", payload.Password, 8, 128, true, true)
	if err != nil {
		return Credentials{}, err
	}

	return Credentials{Identifier: identifier, Password: password}, nil
}
func ParseRefresh(reader io.Reader) (string, error) {
	var payload struct {
		Refresh *string `json:"refresh_token"`
	}
	object, err := Decode(reader, &payload, "refresh_token")
	if err != nil {
		return "", err
	}
	refresh, err := Text(object, "refresh_token", payload.Refresh, 1, 512, false, true)
	if err != nil {
		return "", err
	}

	return refresh, nil
}
func ParseActivity(reader io.Reader) (string, error) {
	var payload struct {
		Page *string `json:"current_page"`
	}
	object, err := Decode(reader, &payload, "current_page")
	if err != nil {
		return "", err
	}
	page, err := Text(object, "current_page", payload.Page, 1, 255, true, true)
	if err != nil {
		return "", err
	}
	if !activityPattern.MatchString(page) {
		return "", Invalid("current_page", "Invalid application path", "string_pattern_mismatch")
	}

	return page, nil
}
func ParseCreateUser(reader io.Reader) (domain.UserCreate, error) {
	var payload struct {
		Superuser bool      `json:"is_superuser"`
		FirstName *string   `json:"first_name"`
		LastName  *string   `json:"last_name"`
		Mobile    *string   `json:"mobile"`
		Username  *string   `json:"username"`
		Email     *string   `json:"email"`
		Roles     *[]string `json:"role_names"`
	}
	object, err := Decode(reader, &payload, "username", "email", "role_names", "first_name", "last_name", "mobile", "is_superuser")
	if err != nil {
		return domain.UserCreate{}, err
	}
	if raw, present := object["is_superuser"]; present && string(raw) == "null" {
		return domain.UserCreate{}, Invalid("is_superuser", "Use true or false", "bool_type")
	}
	username, err := Text(object, "username", payload.Username, 3, 64, true, true)
	if err != nil {
		return domain.UserCreate{}, err
	}
	if !usernamePattern.MatchString(username) {
		return domain.UserCreate{}, Invalid("username", "Invalid username", "string_pattern_mismatch")
	}
	address, err := Text(object, "email", payload.Email, 5, 255, true, true)
	if err != nil {
		return domain.UserCreate{}, err
	}
	if !email(address) {
		return domain.UserCreate{}, Invalid("email", "A valid email address is required", "value_error")
	}
	minimumRoles := 1
	if payload.Superuser {
		minimumRoles = 0
	}
	if err := List("role_names", payload.Roles, minimumRoles, 10, true); err != nil {
		return domain.UserCreate{}, err
	}

	first, e := Text(object, "first_name", payload.FirstName, 1, 100, true, true)
	if e != nil {
		return domain.UserCreate{}, e
	}
	last, e := Text(object, "last_name", payload.LastName, 1, 100, true, true)
	if e != nil {
		return domain.UserCreate{}, e
	}
	mobile, e := Text(object, "mobile", payload.Mobile, 8, 24, true, true)
	if e != nil {
		return domain.UserCreate{}, e
	}
	if !domain.ValidEmployeeContact(first, last, mobile) {
		return domain.UserCreate{}, Invalid("mobile", "Use 8 to 15 digits, optionally with country code and spaces", "value_error")
	}
	return domain.UserCreate{IsSuperuser: payload.Superuser, FirstName: first, LastName: last, Mobile: mobile, Username: username, Email: strings.ToLower(address), RoleNames: *payload.Roles}, nil
}
func ParseUpdateUser(reader io.Reader) (domain.UserUpdate, error) {
	var payload struct {
		Email *string   `json:"email"`
		Roles *[]string `json:"role_names"`
	}
	object, err := Decode(reader, &payload, "email", "role_names")
	if err != nil {
		return domain.UserUpdate{}, err
	}
	if payload.Email != nil {
		address, err := Text(object, "email", payload.Email, 5, 255, true, false)
		if err != nil {
			return domain.UserUpdate{}, err
		}
		if !email(address) {
			return domain.UserUpdate{}, Invalid("email", "A valid email address is required", "value_error")
		}
		address = strings.ToLower(address)
		payload.Email = &address
	}
	if err := List("role_names", payload.Roles, 1, 10, false); err != nil {
		return domain.UserUpdate{}, err
	}

	return domain.UserUpdate{Email: payload.Email, RoleNames: payload.Roles}, nil
}
func ParsePassword(reader io.Reader) (string, error) {
	var payload struct {
		Password *string `json:"password"`
	}
	object, err := Decode(reader, &payload, "password")
	if err != nil {
		return "", err
	}
	password, err := Text(object, "password", payload.Password, 8, 128, false, true)
	if err != nil {
		return "", err
	}
	if !domain.StrongPassword(password) {
		return "", Invalid("password", "Password must include upper, lower, number, and symbol characters", "value_error")
	}

	return password, nil
}
func ParseCreateRole(reader io.Reader) (domain.RoleCreate, error) {
	var payload struct {
		Name        *string   `json:"name"`
		Description *string   `json:"description"`
		Codes       *[]string `json:"permission_codes"`
	}
	object, err := Decode(reader, &payload, "name", "description", "permission_codes")
	if err != nil {
		return domain.RoleCreate{}, err
	}
	name, err := Text(object, "name", payload.Name, 3, 64, true, true)
	if err != nil {
		return domain.RoleCreate{}, err
	}
	if !rolePattern.MatchString(name) {
		return domain.RoleCreate{}, Invalid("name", "Invalid role name", "string_pattern_mismatch")
	}
	required := false
	if _, present := object["description"]; present {
		required = true
	}
	description, err := Text(object, "description", payload.Description, 0, 255, true, required)
	if err != nil {
		return domain.RoleCreate{}, err
	}
	if err := List("permission_codes", payload.Codes, 1, 100, true); err != nil {
		return domain.RoleCreate{}, err
	}

	return domain.RoleCreate{Name: name, Description: description, PermissionCodes: *payload.Codes}, nil
}
func ParseUpdateRole(reader io.Reader) (domain.RoleUpdate, error) {
	var payload struct {
		Description *string   `json:"description"`
		Codes       *[]string `json:"permission_codes"`
		Active      *bool     `json:"is_active"`
	}
	object, err := Decode(reader, &payload, "description", "permission_codes", "is_active")
	if err != nil {
		return domain.RoleUpdate{}, err
	}
	if payload.Description != nil {
		description, err := Text(object, "description", payload.Description, 0, 255, true, false)
		if err != nil {
			return domain.RoleUpdate{}, err
		}
		payload.Description = &description
	}
	if err := List("permission_codes", payload.Codes, 1, 100, false); err != nil {
		return domain.RoleUpdate{}, err
	}

	return domain.RoleUpdate{Description: payload.Description, PermissionCodes: payload.Codes, Active: payload.Active}, nil
}
