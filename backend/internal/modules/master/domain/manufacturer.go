package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/ranakdinesh/epcERP/helpers"
)

type Manufacturer struct {
	ID                   string
	Code, Name           string
	Data                 map[string]any
	Active               bool
	Version              int32
	CreatedAt, UpdatedAt time.Time
	CreatedBy, UpdatedBy *string
}
type ManufacturerData struct {
	Eligible []string
	Notes    string
}

func (d ManufacturerData) Snapshot() map[string]any {
	return map[string]any{"eligible": d.Eligible, "notes": d.Notes}
}

type ManufacturerEvent struct {
	ID, SubjectID, Action, Reason string
	Before, After                 map[string]any
	CreatedAt                     time.Time
	Actor                         *string
}

var ManufacturerUnauthorized = &Error{Kind: Unauthenticated, Detail: "Invalid or expired credentials"}
var ManufacturerForbidden = &Error{Kind: AccessDenied, Detail: "Permission denied"}
var ManufacturerMissing = &Error{Kind: NotFound, Detail: "Record not found"}
var ManufacturerStale = &Error{Kind: Conflict, Detail: "Record changed; refresh before saving"}
var ManufacturerConflict = &Error{Kind: Conflict, Detail: "Duplicate or conflicting record; refresh and review the existing entry"}

func ManufacturerInvalid(s string) error  { return &Error{Kind: InvalidInput, Detail: s} }
func manufacturerWhitespace(r rune) bool  { return unicode.IsSpace(r) }
func ManufacturerTrim(s string) string    { return strings.TrimFunc(s, manufacturerWhitespace) }
func ManufacturerNameKey(s string) string { return helpers.Fold(ManufacturerTrim(s)) }

var manufacturerCodePattern = regexp.MustCompile(`^[A-Z0-9]{1,16}$`)

func ManufacturerValidateCode(s string) (string, error) {
	s = strings.ToUpper(ManufacturerTrim(s))
	if !manufacturerCodePattern.MatchString(s) {
		return "", ManufacturerInvalid("Code requires 1–16 letters or digits")
	}
	return s, nil
}
func ManufacturerValidateEligibility(data ManufacturerData, activeCodes []string) error {
	if len(data.Eligible) == 0 {
		return ManufacturerInvalid("Select at least one category or product type")
	}
	seen := map[string]bool{}
	for _, code := range data.Eligible {
		if seen[code] {
			return ManufacturerInvalid("Duplicate manufacturer eligibility")
		}
		seen[code] = true
	}
	active := map[string]bool{}
	for _, code := range activeCodes {
		active[code] = true
	}
	for _, code := range data.Eligible {
		if !active[code] {
			return ManufacturerInvalid("Manufacturer eligibility must reference active categories or types")
		}
	}
	return nil
}

var manufacturerUuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`)

func ManufacturerCanonicalID(s string) (string, error) {
	s = strings.TrimPrefix(strings.TrimSuffix(s, "}"), "{")
	if strings.HasPrefix(strings.ToLower(s), "urn:uuid:") {
		s = s[9:]
	}
	if !manufacturerUuidPattern.MatchString(s) {
		return "", ManufacturerInvalid("Input should be a valid UUID")
	}
	s = strings.ToLower(strings.ReplaceAll(s, "-", ""))
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}

type ManufacturerChoice struct {
	Code       string
	Name       string
	Kind       string
	ParentName string
	ParentCode string
	Active     bool
}
type ManufacturerSelection struct {
	Rows  []ManufacturerChoice
	Total int64
}
type ManufacturerLegacySource struct {
	CreatedBy *string
	UpdatedBy *string
	ID        string
	Version   int32
	CreatedAt time.Time
	UpdatedAt time.Time
}
type ManufacturerLegacy struct {
	Source ManufacturerLegacySource
	Code   string
	Name   string
	Data   ManufacturerData
	Active *bool
}

func ManufacturerValidateImport(v ManufacturerLegacy) error {
	clean, e := ManufacturerValidateCode(v.Code)
	if e != nil || clean != v.Code {
		return ManufacturerInvalid("Invalid manufacturer code")
	}
	if ManufacturerTrim(v.Name) == "" || len(v.Name) > 255 || len(v.Data.Notes) > 2000 || len(v.Data.Eligible) == 0 || len(v.Data.Eligible) > 200 || v.Active == nil || v.Source.Version < 1 || v.Source.CreatedAt.IsZero() || v.Source.UpdatedAt.IsZero() {
		return ManufacturerInvalid("Invalid manufacturer source snapshot")
	}
	_, e = ManufacturerCanonicalID(v.Source.ID)
	return e
}
