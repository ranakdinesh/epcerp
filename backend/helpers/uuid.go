package helpers

import (
	"errors"
	"regexp"
	"strings"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func CanonicalID(value string) (string, error) {
	if len(value) == 32 {
		value = value[:8] + "-" + value[8:12] + "-" + value[12:16] + "-" + value[16:20] + "-" + value[20:]
	}
	if !uuidPattern.MatchString(value) {
		return "", errors.New("invalid UUID")
	}
	return strings.ToLower(value), nil
}
