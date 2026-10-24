package domain

import "strings"

func RecoveryEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
