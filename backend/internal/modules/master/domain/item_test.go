package domain

import (
	"testing"
)

func itemFixture() (ItemConfiguration, ItemMake) {
	c := ItemConfiguration{ID: "config", Code: "LTPC", Name: "LT Power Cable", Active: true, CategoryID: "cat", Data: ItemDefinition{Kind: "type", CategoryCode: "CAB", FamilyCode: "LTPC", UOM: "M", Fields: []ItemField{{Code: "C", Label: "Cores", Required: true, Options: []ItemOption{{Code: "3C", Label: "3 core"}}}, {Code: "SZ", Label: "Size", Required: true, Options: []ItemOption{{Code: "4", Label: "4 sq mm"}}}, {Code: "SH", Label: "Sheath", Options: []ItemOption{{Code: "__NA__", Label: "N/A"}}}}}}
	m := ItemMake{ID: "make", Code: "POL", Name: "Polycab", Active: true, Data: ItemMakeData{Eligible: []string{"LTPC"}}}
	return c, m
}
func TestItemPythonSKUAndControlledIdentity(t *testing.T) {
	c, m := itemFixture()
	d, e := ItemDerive(c, m, map[string]string{"C": " 3c ", "SZ": "4"}, " AB  42 ")
	if e != nil {
		t.Fatal(e)
	}
	sku, e := ItemIssue(d.Prefix, 1)
	if e != nil || sku != "CAB-LTPC-C3C-SZ4-POL-AB42-001" || d.Model != "ab 42" {
		t.Fatal(sku, d.Model, e)
	}
	if d.Description != "LT Power Cable · Cores: 3 core · Size: 4 sq mm · Polycab · ab 42" {
		t.Fatal(d.Description)
	}
	na, e := ItemDerive(c, m, map[string]string{"C": "3C", "SZ": "4", "SH": "__NA__"}, "")
	if e != nil || na.Specifications["SH"] != "__NA__" {
		t.Fatal("explicit NA", e)
	}
	for _, v := range []map[string]string{{"C": "3C"}, {"C": "3C", "SZ": "BAD"}, {"C": "3C", "SZ": "4", "UNKNOWN": "X"}} {
		if _, e = ItemDerive(c, m, v, ""); e == nil {
			t.Fatal("invalid specifications accepted")
		}
	}
	c.Data.ModelRequired = true
	if _, e = ItemDerive(c, m, map[string]string{"C": "3C", "SZ": "4"}, "  "); e == nil {
		t.Fatal("missing model accepted")
	}
	c.Data.ModelRequired = false
	m.Data.Eligible = []string{"OTHER"}
	if _, e = ItemDerive(c, m, map[string]string{"C": "3C", "SZ": "4"}, ""); e == nil {
		t.Fatal("ineligible accepted")
	}
	m.Data.Eligible = []string{"CAB"}
	if !ItemEligible(c, m) {
		t.Fatal("category eligibility")
	}
	c.Active = false
	if ItemEligible(c, m) {
		t.Fatal("inactive config accepted")
	}
}
func TestItemPermanentCategoryNumbersAndVersions(t *testing.T) {
	if s, e := ItemIssue("CAB-X", 999); e != nil || s != "CAB-X-999" {
		t.Fatal(s, e)
	}
	for _, n := range []int32{0, 1000} {
		if _, e := ItemIssue("CAB-X", n); e == nil {
			t.Fatal("invalid serial")
		}
	}
	v := Item{Status: "approved", Version: 2}
	if e := ItemTransition(v, "retired", 1); e != ItemConflict {
		t.Fatal("stale version")
	}
	if e := ItemTransition(v, "approved", 2); e == nil {
		t.Fatal("same-state update")
	}
	if e := ItemTransition(v, "retired", 2); e != nil {
		t.Fatal(e)
	}
	if _, e := ItemReason(" "); e == nil {
		t.Fatal("missing reason")
	}
}
