package domain

import (
	"strings"
)

type VendorCreate struct {
	Code   string
	Name   string
	Data   map[string]any
	Reason string
}
type VendorUpdate struct {
	Name    string
	Data    map[string]any
	Version int32
	Active  *bool
	Reason  string
}

var VendorStale = &Error{Kind: Conflict, Detail: "Supplier changed; refresh before editing"}

func VendorValidateReason(s string) error {
	if strings.TrimSpace(s) == "" || len(s) > 2000 {
		return VendorFieldError("reason", "A change reason is required (maximum 2000 bytes)")
	}
	return nil
}

func VendorNormalizeCode(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }
func VendorNormalizeName(s string) string { return strings.TrimSpace(s) }
