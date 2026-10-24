package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/ranakdinesh/epcERP/helpers"
)

type Reference struct {
	ID, Name, Code, NormalizedName, StateID string
	Active                                  bool
	CreatedAt, UpdatedAt                    time.Time
	CreatedBy, UpdatedBy                    *string
}
type ReferenceInput struct{ Name, Code, StateID string }

var ReferenceUnauthorized = &Error{Kind: Unauthenticated, Detail: "Invalid or expired credentials"}
var ReferenceForbidden = &Error{Kind: AccessDenied, Detail: "Insufficient permission"}
var ReferenceUnknown = &Error{Kind: NotFound, Detail: "Unknown reference master"}
var ReferenceMissing = &Error{Kind: NotFound, Detail: "Master value not found"}
var ReferenceDuplicate = &Error{Kind: Conflict, Detail: "This master value already exists"}

func ReferenceInvalid(s string) error { return &Error{Kind: InvalidInput, Detail: s} }
func ReferenceValidKind(k string) bool {
	return k == "client-types" || k == "industries" || k == "states" || k == "cities"
}
func ReferencePermission(k string, write bool) string {
	resource := "locations"
	if k == "client-types" {
		resource = "client_types"
	}
	if k == "industries" {
		resource = "industries"
	}
	action := "read"
	if write {
		action = "write"
	}
	return "settings." + resource + "." + action
}
func referenceWhitespace(r rune) bool { return unicode.IsSpace(r) }
func ReferenceTrim(s string) string   { return strings.TrimFunc(s, referenceWhitespace) }
func ReferenceName(s string) string {
	return strings.Join(strings.FieldsFunc(s, referenceWhitespace), " ")
}
func ReferenceNormalize(s string) string { return helpers.Fold(ReferenceName(s)) }

var referenceCodePattern = regexp.MustCompile(`[^A-Z0-9]+`)

func ReferenceCode(s string) string {
	s = strings.Trim(referenceCodePattern.ReplaceAllString(strings.ToUpper(ReferenceTrim(s)), "-"), "-")
	if len(s) > 32 {
		s = s[:32]
	}
	return s
}
func ReferencePrepare(k string, in ReferenceInput) (ReferenceInput, error) {
	in.Name = ReferenceName(in.Name)
	if in.Name == "" {
		return in, ReferenceInvalid("A name is required")
	}
	code := in.Code
	if code == "" {
		code = in.Name
	}
	in.Code = ReferenceCode(code)
	if in.Code == "" {
		return in, ReferenceInvalid("A valid code is required")
	}
	if k == "cities" && in.StateID == "" {
		return in, ReferenceInvalid("A city must be linked to a state or union territory")
	}
	if k != "cities" && in.StateID != "" {
		return in, ReferenceInvalid("Only cities may be linked to a state or union territory")
	}
	return in, nil
}

var referenceUuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`)

func ReferenceCanonicalID(s string) (string, error) {
	s = strings.TrimPrefix(strings.TrimSuffix(s, "}"), "{")
	if strings.HasPrefix(strings.ToLower(s), "urn:uuid:") {
		s = s[9:]
	}
	if !referenceUuidPattern.MatchString(s) {
		return "", ReferenceInvalid("Input should be a valid UUID")
	}
	s = strings.ToLower(strings.ReplaceAll(s, "-", ""))
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}
