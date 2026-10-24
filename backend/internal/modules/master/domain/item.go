package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

func ItemInvalid(field, detail string) error {
	return &Error{Kind: InvalidInput, Detail: detail, Field: field}
}

var ItemUnauthorized = &Error{Kind: Unauthenticated, Detail: "Invalid or expired credentials", Field: ""}
var ItemForbidden = &Error{Kind: AccessDenied, Detail: "Permission denied", Field: ""}
var ItemMissing = &Error{Kind: NotFound, Detail: "Item or reference not found", Field: ""}
var ItemConflict = &Error{Kind: Conflict, Detail: "Record changed; reload before saving", Field: ""}

type ItemOption struct {
	Code  string
	Label string
}
type ItemField struct {
	Code     string
	Label    string
	Required bool
	Options  []ItemOption
}
type ItemConfiguration struct {
	ID            string
	Code          string
	Name          string
	Active        bool
	Version       int32
	CategoryID    string
	SubcategoryID string
	Data          ItemDefinition
}
type ItemDefinition struct {
	Kind          string
	CategoryCode  string
	FamilyCode    string
	UOM           string
	Revision      int32
	ModelRequired bool
	Fields        []ItemField
}
type ItemMake struct {
	ID     string
	Code   string
	Name   string
	Active bool
	Data   ItemMakeData
}
type ItemMakeData struct {
	Eligible []string
}
type ItemReferences struct {
	Catalogue     []ItemConfiguration
	Manufacturers []ItemMake
}
type ItemFrozenConfiguration struct {
	ItemDefinition
	Name string
	Code string
}
type ItemSnapshot struct {
	Configuration ItemFrozenConfiguration
	Manufacturer  ItemMake
	Prefix        string
}
type Item struct {
	ID              string
	ConfigurationID string
	ManufacturerID  string
	CategoryID      string
	Model           string
	Specifications  map[string]string
	Snapshot        ItemSnapshot
	Fingerprint     string
	EngineeringKey  string
	Description     string
	SKU             string
	Serial          int32
	Status          string
	Active          bool
	Version         int32
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CreatedBy       *string
	UpdatedBy       *string
}
type ItemEvent struct {
	ID        string
	SubjectID string
	Action    string
	Reason    string
	Before    *Item
	After     Item
	CreatedAt time.Time
	Actor     string
}
type ItemIdentity struct {
	Family, UOM         string
	Specifications      map[string]string
	Manufacturer, Model string
}
type ItemDerived struct {
	Identity                   ItemIdentity
	Model, Prefix, Description string
	Specifications             map[string]string
}

var itemIdPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ItemCanonicalID(id string) (string, error) {
	if !itemIdPattern.MatchString(id) {
		return "", ItemInvalid("", "Input should be a valid UUID")
	}
	return strings.ToLower(id), nil
}
func ItemReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) < 1 || utf8.RuneCountInString(s) > 2000 {
		return "", ItemInvalid("reason", "Provide a reason between 1 and 2000 characters")
	}
	return s, nil
}
func ItemEligible(c ItemConfiguration, m ItemMake) bool {
	if !c.Active || !m.Active {
		return false
	}
	for _, v := range m.Data.Eligible {
		if v == c.Code || v == c.Data.FamilyCode || v == c.Data.CategoryCode {
			return true
		}
	}
	return false
}

var itemModelPattern = regexp.MustCompile(`[^A-Z0-9]`)

// Derive retains Python's field order, human-readable SKU and all-filled-field identity.
func ItemDerive(c ItemConfiguration, m ItemMake, spec map[string]string, model string) (ItemDerived, error) {
	if !c.Active || c.Data.Kind != "type" {
		return ItemDerived{}, ItemInvalid("configuration_id", "Choose an active configuration with an active category and unit")
	}
	if !ItemEligible(c, m) {
		return ItemDerived{}, ItemInvalid("manufacturer_id", "Manufacturer is inactive or not eligible for this configuration")
	}
	if len(spec) > 64 {
		return ItemDerived{}, ItemInvalid("specifications", "Too many specification fields")
	}
	known := map[string]bool{}
	for _, f := range c.Data.Fields {
		known[f.Code] = true
	}
	for k := range spec {
		if !known[k] {
			return ItemDerived{}, ItemInvalid("specifications", "Unknown or inactive specification field")
		}
	}
	values := map[string]string{}
	tokens := []string{c.Data.CategoryCode, c.Data.FamilyCode}
	labels := []string{c.Name}
	for _, f := range c.Data.Fields {
		v := strings.ToUpper(strings.TrimSpace(spec[f.Code]))
		if v == "" {
			if f.Required {
				return ItemDerived{}, ItemInvalid(f.Code, "This specification is required")
			}
			continue
		}
		found := false
		for _, o := range f.Options {
			if o.Code == v {
				found = true
				labels = append(labels, f.Label+": "+o.Label)
				break
			}
		}
		if !found {
			return ItemDerived{}, ItemInvalid(f.Code, "Choose a valid active option")
		}
		values[f.Code] = v
		tokens = append(tokens, f.Code+v)
	}
	model = strings.ToLower(strings.Join(strings.Fields(model), " "))
	if utf8.RuneCountInString(model) > 120 {
		return ItemDerived{}, ItemInvalid("model", "Model must be at most 120 characters")
	}
	if c.Data.ModelRequired && model == "" {
		return ItemDerived{}, ItemInvalid("model", "Model or part number is required")
	}
	tokens = append(tokens, m.Code)
	labels = append(labels, m.Name)
	if model != "" {
		mt := itemModelPattern.ReplaceAllString(strings.ToUpper(model), "")
		if len(mt) > 24 {
			mt = mt[:24]
		}
		if mt != "" {
			tokens = append(tokens, mt)
		}
		labels = append(labels, model)
	}
	prefix := strings.Join(tokens, "-")
	if len(prefix) > 900 {
		return ItemDerived{}, ItemInvalid("specifications", "Specifications produce a SKU prefix longer than 900 characters")
	}
	return ItemDerived{Identity: ItemIdentity{Family: c.Data.FamilyCode, UOM: c.Data.UOM, Specifications: values, Manufacturer: m.ID, Model: model}, Model: model, Prefix: prefix, Description: strings.Join(labels, " · "), Specifications: values}, nil
}
func ItemIssue(prefix string, n int32) (string, error) {
	if n < 1 || n > 999 {
		return "", ItemInvalid("", "All 999 SKU numbers in this category have been issued; numbering policy must be extended")
	}
	return fmt.Sprintf("%s-%03d", prefix, n), nil
}
func ItemTransition(v Item, status string, version int32) error {
	if version != v.Version {
		return ItemConflict
	}
	if !((v.Status == "approved" && status == "retired") || (v.Status == "retired" && status == "approved")) {
		return ItemInvalid("status", "Invalid item status transition")
	}
	return nil
}
