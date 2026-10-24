package domain

import "strings"

func Username(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 3 || len(value) > 64 {
		return "", &Error{Invalid, "Use 3 to 64 letters, numbers, dots, underscores or hyphens."}
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '.' && c != '_' && c != '-' {
			return "", &Error{Invalid, "Use 3 to 64 letters, numbers, dots, underscores or hyphens."}
		}
	}
	return value, nil
}
func UsernameTaken(name string) error {
	return &Error{Conflict, name + " already exists. Please choose a different username."}
}

var ErrUsernameStale = &Error{Conflict, "Your username changed. Close and reopen your profile before saving."}
