package domain

import (
	"testing"
)

func templateFixture() Template {
	return Template{SubcategoryID: "scope", Active: true, Fields: []TemplateField{{Code: "C", Mandatory: true, Active: true, AllowNA: true, UseDescription: true, DescriptionOrder: 2, Prefix: "X ", UseDuplicate: true, Options: []TemplateOption{{Code: "A", Label: "Copper", Active: true}, {Code: "B", Label: "Aluminium", Active: true}, {Code: "OLD", Label: "Old", Active: false}}}, {Code: "SZ", Mandatory: true, Active: true, UseDescription: true, DescriptionOrder: 1, Suffix: " mm", UseDuplicate: true, Options: []TemplateOption{{Code: "S", Label: "4", Active: true}}}, {Code: "EXTRA", Active: true, UseDescription: true, DescriptionOrder: 3, Options: []TemplateOption{{Code: "E", Label: "Extra", Active: true}}}, {Code: "INACTIVE", Active: false}}}
}
func TestTemplateNAAndDescriptionIdentity(t *testing.T) {
	v := templateFixture()
	p, e := TemplateEvaluate(v, map[string]string{"C": "A", "SZ": "S"})
	if e != nil || p.Description != "4 mm X Copper" {
		t.Fatal(p, e)
	}
	extra, e := TemplateEvaluate(v, map[string]string{"C": "A", "SZ": "S", "EXTRA": "E"})
	if e != nil || extra.Identity.Values["C"] != p.Identity.Values["C"] || extra.Identity.Values["EXTRA"] != "" || extra.Description == p.Description {
		t.Fatal("nonidentity field affects engineering identity", e)
	}
	na, e := TemplateEvaluate(v, map[string]string{"C": TemplateNA, "SZ": "S"})
	if e != nil || na.Description != "4 mm" || na.Identity.Values["C"] == p.Identity.Values["C"] {
		t.Fatal("explicit NA semantics", e)
	}
	v.Fields[0].Mandatory = false
	blank, e := TemplateEvaluate(v, map[string]string{"SZ": "S"})
	if e != nil || blank.Identity.Values["C"] == na.Identity.Values["C"] {
		t.Fatal("NA conflated with blank", e)
	}
	v.SubcategoryID = "other"
	scoped, e := TemplateEvaluate(v, map[string]string{"SZ": "S"})
	if e != nil || scoped.Identity.Subcategory == blank.Identity.Subcategory {
		t.Fatal("identity not scoped", e)
	}
}
func TestTemplateInvalidSelections(t *testing.T) {
	for _, values := range []map[string]string{{"C": "", "SZ": "S"}, {"C": "OLD", "SZ": "S"}, {"C": "A", "SZ": TemplateNA}, {"C": "A", "SZ": "S", "INACTIVE": "anything"}, {"C": "A", "SZ": "S", "UNKNOWN": "anything"}} {
		if _, e := TemplateEvaluate(templateFixture(), values); e == nil {
			t.Fatal("invalid selection accepted")
		}
	}
}
func TestTemplateDefinitionValidation(t *testing.T) {
	v := templateFixture()
	v.Fields = v.Fields[:2]
	for i := range v.Fields {
		v.Fields[i].Name = v.Fields[i].Code
		v.Fields[i].InputType = "dropdown"
		v.Fields[i].SortOrder = int32(i + 1)
		for j := range v.Fields[i].Options {
			v.Fields[i].Options[j].SortOrder = int32(j + 1)
		}
	}
	d := TemplateDefinition{SubcategoryCode: "LTPC", Name: "Cable", Fields: v.Fields}
	if e := TemplateValidate([]TemplateDefinition{d}); e != nil {
		t.Fatal(e)
	}
	d.Fields[1].DescriptionOrder = d.Fields[0].DescriptionOrder
	if e := TemplateValidate([]TemplateDefinition{d}); e == nil {
		t.Fatal("duplicate description order accepted")
	}
}

func TestTemplateNativeRevisionIdentityAndReason(t *testing.T) {
	before := Template{Revision: 2, Fields: []TemplateField{{Code: "C", Options: []TemplateOption{{Code: "1"}}}}}
	def := TemplateDefinition{Fields: []TemplateField{{Code: "C", Options: []TemplateOption{{Code: "1", Active: false}, {Code: "2", Active: true}}}}}
	if e := TemplateValidateRevision(before, def, 2); e != nil {
		t.Fatal(e)
	}
	if e := TemplateValidateRevision(before, def, 1); e == nil {
		t.Fatal("stale revision accepted")
	}
	def.Fields[0].Options = def.Fields[0].Options[1:]
	if e := TemplateValidateRevision(before, def, 2); e == nil {
		t.Fatal("removed option accepted")
	}
	for _, v := range []struct {
		revision int32
		reason   string
	}{{-1, "reason"}, {2147483647, "reason"}, {0, "  "}} {
		if e := TemplateValidatePublication(v.revision, v.reason); e == nil {
			t.Fatal("invalid publication accepted")
		}
	}
	if e := TemplateValidatePublication(0, "Initial definition"); e != nil {
		t.Fatal(e)
	}
}
