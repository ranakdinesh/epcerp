package domain

import (
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
	"testing"
	"time"
)

func categoryFixture() []CategorySource {
	now := time.Now()
	return []CategorySource{{ID: "11111111-1111-4111-8111-111111111111", Code: "CAB", Name: "Cables", Active: true, Version: 1, CreatedAt: now, UpdatedAt: now, Data: CategorySourceData{Kind: "category"}}, {ID: "22222222-2222-4222-8222-222222222222", Code: "LTPC", Name: "LT Power Cable", Active: true, Version: 1, CreatedAt: now, UpdatedAt: now, Data: CategorySourceData{Kind: "type", CategoryCode: "CAB", UOM: "Meter", FamilyCode: "LTPC", Revision: 1, Fields: []CategoryField{{Code: "SZ", Label: "Size", Required: true, Options: []CategoryOption{{Code: "4SQMM", Label: "4 sqmm"}}}}}}}
}
func TestCategoryImportValidation(t *testing.T) {
	if e := CategoryValidate(categoryFixture()); e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name string
		edit func([]CategorySource)
	}{{"orphan", func(r []CategorySource) { r[1].Data.CategoryCode = "MISSING" }}, {"unit", func(r []CategorySource) { r[1].Data.UOM = "Free text" }}, {"duplicate", func(r []CategorySource) { r[1].Code = "CAB" }}, {"invalid-id", func(r []CategorySource) { r[0].ID = "bad" }}, {"empty-options", func(r []CategorySource) { r[1].Data.Fields[0].Options = nil }}, {"option-label-duplicate", func(r []CategorySource) {
		r[1].Data.Fields[0].Options = append(r[1].Data.Fields[0].Options, CategoryOption{Code: "NEW", Label: "4 SQMM"})
	}}, {"empty-name", func(r []CategorySource) { r[0].Name = " " }}, {"unknown-kind", func(r []CategorySource) { r[1].Data.Kind = "item" }}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := categoryFixture()
			c.edit(r)
			if CategoryValidate(r) == nil {
				t.Fatal("invalid import accepted")
			}
		})
	}
}
func TestCategoryPermissions(t *testing.T) {
	if (sharedIdentity.Principal{Active: true, ID: "actor", Permissions: []string{"settings.categories.view"}}).Allows("settings.categories.import") {
		t.Fatal("view grants import")
	}
	if !(sharedIdentity.Principal{Active: true, ID: "actor", Superuser: true}).Allows("future.permission") {
		t.Fatal("recovery bypass missing")
	}
	if (sharedIdentity.Principal{ID: "actor", Superuser: true}).Allows("settings.categories.read") {
		t.Fatal("inactive recovery account allowed")
	}
}

func TestCategoryNativeCreationAndUniformCategoryUnits(t *testing.T) {
	if _, e := CategoryCleanCreate("category", CategoryCreate{Code: "cab", Name: " Cables  and wires ", UnitID: "11111111-1111-4111-8111-111111111111", Reason: "Setup"}); e != nil {
		t.Fatal(e)
	}
	for _, in := range []CategoryCreate{{Code: "BAD-", Name: "Name", Reason: "Setup"}, {Code: "UNIT", Name: "", Reason: "Setup"}, {Code: "UNIT", Name: "Name", Reason: ""}} {
		if _, e := CategoryCleanCreate("unit", in); e == nil {
			t.Fatal("invalid creation accepted")
		}
	}
	r := categoryFixture()
	extra := r[1]
	extra.Code = "HTC"
	extra.ID = "33333333-3333-4333-8333-333333333333"
	extra.Data.UOM = "Nos"
	if _, e := CategoryUnits(append(r, extra)); e == nil {
		t.Fatal("mixed category units accepted")
	}
}
