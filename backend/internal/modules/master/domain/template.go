package domain

import (
	"regexp"
	"sort"
	"strings"
)

var TemplateUnauthorized = &Error{Kind: Unauthenticated, Detail: "Invalid or expired credentials"}
var TemplateForbidden = &Error{Kind: AccessDenied, Detail: "Permission denied"}
var TemplateMissing = &Error{Kind: NotFound, Detail: "Configuration template pending"}
var TemplateConflict = &Error{Kind: Conflict, Detail: "Existing template differs; publish a reviewed revision instead"}

func TemplateInvalid(detail string) error { return &Error{Kind: InvalidInput, Detail: detail} }

type TemplateOption struct {
	ID        string
	Code      string
	Label     string
	Active    bool
	SortOrder int32
}
type TemplateField struct {
	ID               string
	Code             string
	Name             string
	InputType        string
	Mandatory        bool
	AllowNA          bool
	UseDescription   bool
	DescriptionOrder int32
	Prefix           string
	Suffix           string
	UseDuplicate     bool
	SortOrder        int32
	Active           bool
	Options          []TemplateOption
}
type TemplateDefinition struct {
	ModelRequired   bool
	SubcategoryCode string
	Name            string
	Fields          []TemplateField
}
type Template struct {
	ModelRequired bool
	ID            string
	SubcategoryID string
	Name          string
	Revision      int32
	Active        bool
	Fields        []TemplateField
	Fingerprint   string
}
type TemplateSubcategory struct {
	ID     string
	Active bool
}
type TemplateResult struct {
	Created   int
	Unchanged int
}

const TemplateNA = "__NA__"

var templateCodePattern = regexp.MustCompile(`^[A-Z0-9]{1,32}$`)
var templateUuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func TemplateValidateID(id string) error {
	if !templateUuidPattern.MatchString(id) {
		return TemplateInvalid("Invalid subcategory identity")
	}
	return nil
}
func TemplateValidate(defs []TemplateDefinition) error {
	if len(defs) == 0 || len(defs) > 20 {
		return TemplateInvalid("Provide 1–20 templates")
	}
	parents := map[string]bool{}
	names := map[string]bool{}
	for _, d := range defs {
		if !templateCodePattern.MatchString(d.SubcategoryCode) || parents[d.SubcategoryCode] || strings.TrimSpace(d.Name) == "" || len(d.Name) > 255 || names[strings.ToLower(d.Name)] {
			return TemplateInvalid("Invalid or duplicate template name/subcategory")
		}
		parents[d.SubcategoryCode] = true
		names[strings.ToLower(d.Name)] = true
		if len(d.Fields) == 0 || len(d.Fields) > 64 {
			return TemplateInvalid("Provide 1–64 fields")
		}
		codes := map[string]bool{}
		fieldNames := map[string]bool{}
		orders := map[int32]bool{}
		descriptionOrders := map[int32]bool{}
		totalOptions := 0
		for _, f := range d.Fields {
			if f.ID != "" || !templateCodePattern.MatchString(f.Code) || codes[f.Code] || strings.TrimSpace(f.Name) == "" || len(f.Name) > 255 || fieldNames[strings.ToLower(f.Name)] || f.InputType != "dropdown" || f.SortOrder < 1 || f.SortOrder > 10000 || orders[f.SortOrder] || len(f.Prefix) > 128 || len(f.Suffix) > 128 {
				return TemplateInvalid("Invalid or duplicate field")
			}
			codes[f.Code] = true
			fieldNames[strings.ToLower(f.Name)] = true
			orders[f.SortOrder] = true
			if f.UseDescription {
				if f.DescriptionOrder < 1 || f.DescriptionOrder > 10000 || descriptionOrders[f.DescriptionOrder] {
					return TemplateInvalid("Description orders must be positive and unique")
				}
				descriptionOrders[f.DescriptionOrder] = true
			} else if f.DescriptionOrder != 0 {
				return TemplateInvalid("Excluded descriptions must have order zero")
			}
			if len(f.Options) == 0 || len(f.Options) > 128 {
				return TemplateInvalid("Provide 1–128 controlled options")
			}
			totalOptions += len(f.Options)
			if totalOptions > 2048 {
				return TemplateInvalid("Too many template options")
			}
			optionCodes := map[string]bool{}
			labels := map[string]bool{}
			optionOrders := map[int32]bool{}
			activeOptions := 0
			for _, o := range f.Options {
				if o.ID != "" || !templateCodePattern.MatchString(o.Code) || optionCodes[o.Code] || strings.TrimSpace(o.Label) == "" || len(o.Label) > 255 || labels[strings.ToLower(o.Label)] || o.SortOrder < 1 || o.SortOrder > 10000 || optionOrders[o.SortOrder] {
					return TemplateInvalid("Invalid or duplicate controlled option")
				}
				optionCodes[o.Code] = true
				labels[strings.ToLower(o.Label)] = true
				optionOrders[o.SortOrder] = true
				if o.Active {
					activeOptions++
				}
			}
			if f.Active && activeOptions == 0 {
				return TemplateInvalid("Active fields require active controlled options")
			}
		}
	}
	return nil
}

type TemplateIdentity struct {
	Subcategory string
	Values      map[string]string
}
type TemplatePreview struct {
	Identity       TemplateIdentity
	Description    string
	EngineeringKey string
}

func TemplateEvaluate(t Template, values map[string]string) (TemplatePreview, error) {
	if !t.Active {
		return TemplatePreview{}, TemplateInvalid("Template is inactive")
	}
	known := map[string]bool{}
	identity := map[string]string{}
	type fragment struct {
		order int32
		text  string
	}
	parts := []fragment{}
	for _, f := range t.Fields {
		if !f.Active {
			continue
		}
		known[f.Code] = true
		value := values[f.Code]
		label := ""
		if value == "" {
			if f.Mandatory {
				return TemplatePreview{}, &Error{Kind: InvalidInput, Field: f.Code, Detail: "Select a value or allowed N/A"}
			}
		} else if value == TemplateNA {
			if !f.AllowNA {
				return TemplatePreview{}, &Error{Kind: InvalidInput, Field: f.Code, Detail: "N/A is not allowed"}
			}
		} else {
			found := false
			for _, o := range f.Options {
				if o.Code == value && o.Active {
					label = o.Label
					found = true
					break
				}
			}
			if !found {
				return TemplatePreview{}, &Error{Kind: InvalidInput, Field: f.Code, Detail: "Select an active controlled option"}
			}
		}
		if f.UseDuplicate {
			identity[f.Code] = value
		}
		if f.UseDescription && label != "" {
			parts = append(parts, fragment{f.DescriptionOrder, f.Prefix + label + f.Suffix})
		}
	}
	for code := range values {
		if !known[code] {
			return TemplatePreview{}, TemplateInvalid("Unknown or inactive specification field")
		}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].order < parts[j].order })
	labels := []string{}
	for _, p := range parts {
		labels = append(labels, p.text)
	}
	return TemplatePreview{Description: strings.Join(labels, " "), Identity: TemplateIdentity{t.SubcategoryID, identity}}, nil
}

func TemplateValidatePublication(expected int32, reason string) error {
	if len(strings.TrimSpace(reason)) == 0 || len(reason) > 1000 {
		return TemplateInvalid("Review reason must contain 1 to 1000 characters")
	}
	if expected < 0 || expected >= 2147483647 {
		return TemplateInvalid("Expected revision is out of range")
	}
	return nil
}
func TemplateValidateRevision(before Template, def TemplateDefinition, expected int32) error {
	if before.Revision != expected {
		return &Error{Kind: Conflict, Detail: "Configuration changed. Reopen the editor before saving."}
	}
	for _, old := range before.Fields {
		var found *TemplateField
		for i := range def.Fields {
			if def.Fields[i].Code == old.Code {
				found = &def.Fields[i]
				break
			}
		}
		if found == nil {
			return TemplateInvalid("Retain existing field codes; deactivate fields instead of removing them")
		}
		for _, option := range old.Options {
			exists := false
			for _, next := range found.Options {
				if next.Code == option.Code {
					exists = true
					break
				}
			}
			if !exists {
				return TemplateInvalid("Retain existing option codes; deactivate options instead of removing them")
			}
		}
	}
	return nil
}
