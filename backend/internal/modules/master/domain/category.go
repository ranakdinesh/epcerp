package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var CategoryUnauthorized = &Error{Kind: Unauthenticated, Detail: "Invalid or expired credentials"}
var CategoryForbidden = &Error{Kind: AccessDenied, Detail: "Permission denied"}
var CategoryConflict = &Error{Kind: Conflict, Detail: "Existing catalogue conflicts with source; review before import"}

func CategoryInvalid(s string) error { return &Error{Kind: InvalidInput, Detail: s} }

type CategoryOption struct {
	Code  string
	Label string
}
type CategoryField struct {
	Code     string
	Label    string
	Required bool
	Options  []CategoryOption
}
type CategoryConfiguration struct {
	Fields        []CategoryField
	ModelRequired bool
	FamilyCode    string
	Revision      int
}
type CategorySourceData struct {
	Kind               string
	CategoryCode       string
	UOM                string
	Fields             []CategoryField
	ModelRequired      bool
	FamilyCode         string
	Revision           int
	Source             string
	LegacyCategoryCode string
}
type CategorySource struct {
	ID        string
	Code      string
	Name      string
	Data      CategorySourceData
	Active    bool
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}
type Category struct {
	ID            string
	Code          string
	Name          string
	Active        bool
	Version       int32
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CreatedBy     *string
	UpdatedBy     *string
	CategoryID    string
	UnitID        string
	Configuration *CategoryConfiguration
	Fingerprint   string
}
type CategoryCatalogue struct {
	Categories    []Category
	Subcategories []Category
	Units         []Category
}
type CategoryImportRecord struct {
	Record Category
	Source *CategorySource
	Actor  string
	Reason string
}
type CategoryResult struct{ Categories, Subcategories, Units int }

var categoryCode = regexp.MustCompile(`^[A-Z0-9]{1,32}$`)
var categoryUuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func categoryValidText(s string, max int) bool {
	return strings.TrimSpace(s) == s && s != "" && utf8.RuneCountInString(s) <= max && !strings.ContainsAny(s, "\x00\r\n")
}
func CategoryValidID(s string) bool { return categoryUuid.MatchString(s) }

var categoryUnits = map[string]string{"Nos": "Number", "Meter": "Meter", "Kg": "Kilogram", "Set": "Set", "Pair": "Pair", "Lot": "Lot", "Sqm": "Square meter", "Rmt": "Running meter", "Bundle": "Bundle", "Box": "Box"}

func CategoryUnitName(code string) string { return categoryUnits[code] }
func CategoryValidate(rows []CategorySource) error {
	if len(rows) == 0 || len(rows) > 1000 {
		return CategoryInvalid("Import requires 1–1000 definitions")
	}
	codes := map[string]bool{}
	ids := map[string]bool{}
	cats := map[string]bool{}
	names := map[string]bool{}
	for _, r := range rows {
		if !CategoryValidID(r.ID) || !categoryCode.MatchString(r.Code) || !categoryValidText(r.Name, 255) || r.Version < 1 || r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() {
			return CategoryInvalid("Invalid source identifier, code, name or audit timestamps")
		}
		if codes[r.Code] || ids[strings.ToLower(r.ID)] {
			return CategoryInvalid("Duplicate source code or identifier")
		}
		codes[r.Code] = true
		ids[strings.ToLower(r.ID)] = true
		if r.Data.Kind == "category" {
			key := strings.ToLower(r.Name)
			if names[key] {
				return CategoryInvalid("Duplicate category name")
			}
			names[key] = true
			cats[r.Code] = true
		} else if r.Data.Kind != "type" {
			return CategoryInvalid("Unknown source definition kind")
		}
	}
	for _, r := range rows {
		d := r.Data
		if d.Kind != "type" {
			continue
		}
		if !cats[d.CategoryCode] || CategoryUnitName(d.UOM) == "" || !categoryCode.MatchString(d.FamilyCode) || d.Revision < 1 || len(d.Fields) == 0 || len(d.Fields) > 64 {
			return CategoryInvalid("Subcategory needs an imported parent, controlled unit and valid configuration")
		}
		fields := map[string]bool{}
		for _, f := range d.Fields {
			if !categoryCode.MatchString(f.Code) || !categoryValidText(f.Label, 255) || fields[f.Code] || len(f.Options) == 0 || len(f.Options) > 500 {
				return CategoryInvalid("Invalid or duplicate configuration field")
			}
			fields[f.Code] = true
			options := map[string]bool{}
			labels := map[string]bool{}
			for _, o := range f.Options {
				key := strings.ToLower(o.Label)
				if !categoryCode.MatchString(o.Code) || !categoryValidText(o.Label, 255) || options[o.Code] || labels[key] {
					return CategoryInvalid("Invalid or duplicate configuration option")
				}
				options[o.Code] = true
				labels[key] = true
			}
		}
	}
	return nil
}

type CategoryCreate struct {
	Code       string
	Name       string
	UnitID     string
	CategoryID string
	Reason     string
}

func CategoryFieldInvalid(field, detail string) error {
	return &Error{Kind: InvalidInput, Detail: detail, Field: field}
}
func CategoryCleanCreate(kind string, in CategoryCreate) (CategoryCreate, error) {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	in.Reason = strings.TrimSpace(in.Reason)
	if !categoryCode.MatchString(in.Code) || (kind == "unit" && len(in.Code) > 16) {
		return in, CategoryFieldInvalid("code", "Use letters and digits, up to 32 characters (16 for units)")
	}
	if !categoryValidText(in.Name, 255) || (kind == "unit" && utf8.RuneCountInString(in.Name) > 128) {
		return in, CategoryFieldInvalid("name", "A valid name is required")
	}
	if in.Reason == "" || utf8.RuneCountInString(in.Reason) > 2000 || strings.ContainsRune(in.Reason, 0) {
		return in, CategoryFieldInvalid("reason", "A reason of up to 2000 characters is required")
	}
	switch kind {
	case "category":
		if !CategoryValidID(in.UnitID) || in.CategoryID != "" {
			return in, CategoryFieldInvalid("unit_id", "Select an active base unit")
		}
	case "subcategory":
		if !CategoryValidID(in.CategoryID) || in.UnitID != "" {
			return in, CategoryFieldInvalid("category_id", "Select an active category; its unit is inherited")
		}
	case "unit":
		if in.UnitID != "" || in.CategoryID != "" {
			return in, CategoryInvalid("Units do not have parent references")
		}
	default:
		return in, CategoryInvalid("Unknown catalogue record kind")
	}
	return in, nil
}
func CategoryUnits(rows []CategorySource) (map[string]string, error) {
	out := map[string]string{}
	for _, s := range rows {
		if s.Data.Kind != "type" {
			continue
		}
		old := out[s.Data.CategoryCode]
		if old != "" && old != s.Data.UOM {
			return nil, CategoryInvalid("All subcategories must share their category base unit")
		}
		out[s.Data.CategoryCode] = s.Data.UOM
	}
	for _, s := range rows {
		if s.Data.Kind == "category" && out[s.Code] == "" {
			return nil, CategoryInvalid("Imported category needs a base unit from its subcategories")
		}
	}
	return out, nil
}
func CategorySameName(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}

var CategoryStale = &Error{Kind: Conflict, Detail: "Record changed; reload before saving"}
var CategoryMissing = &Error{Kind: NotFound, Detail: "Record not found"}

type CategoryEdit struct {
	Code    string
	Name    string
	Reason  string
	Version int32
}
type CategoryCorrection struct {
	Before, After Category
	Actor, Reason string
}

func CategoryCleanEdit(kind string, in CategoryEdit) (CategoryEdit, error) {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	in.Reason = strings.TrimSpace(in.Reason)
	if kind != "category" && kind != "subcategory" && kind != "unit" {
		return in, CategoryInvalid("Unknown record kind")
	}
	if !categoryCode.MatchString(in.Code) || (kind == "unit" && len(in.Code) > 16) {
		return in, CategoryFieldInvalid("code", "Use letters and digits, up to 32 characters (16 for units)")
	}
	if !categoryValidText(in.Name, 255) || (kind == "unit" && utf8.RuneCountInString(in.Name) > 128) {
		return in, CategoryFieldInvalid("name", "A valid name is required")
	}
	if in.Reason == "" || utf8.RuneCountInString(in.Reason) > 2000 || strings.ContainsRune(in.Reason, 0) {
		return in, CategoryFieldInvalid("reason", "A reason of up to 2000 characters is required")
	}
	if in.Version < 1 || in.Version == 2147483647 {
		return in, CategoryFieldInvalid("version", "A current record version is required")
	}
	return in, nil
}
