package repository

import (
	"testing"

	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

// Expected values were captured from Python configured_identity/sku_number.
func TestItemPythonModelNormalizationAndSKU(t *testing.T) {
	values := ItemValues{}
	c := masterDomain.ItemConfiguration{Code: "LTPC", Name: "LT Power Cable", Active: true, Data: masterDomain.ItemDefinition{Kind: "type", CategoryCode: "CAB", FamilyCode: "LTPC", UOM: "M", Fields: []masterDomain.ItemField{{Code: "C", Label: "Cores", Required: true, Options: []masterDomain.ItemOption{{Code: "3C", Label: "3 core"}}}}}}
	m := masterDomain.ItemMake{ID: "make", Code: "POL", Name: "Polycab", Active: true, Data: masterDomain.ItemMakeData{Eligible: []string{"CAB"}}}
	for _, tc := range []struct{ input, model, sku string }{
		{" AB  42 ", "ab 42", "CAB-LTPC-C3C-POL-AB42-001"},
		{"Straße", "strasse", "CAB-LTPC-C3C-POL-STRASSE-001"},
		{"ＡＢ４２", "ab42", "CAB-LTPC-C3C-POL-AB42-001"},
		{"① \x1c K", "1 k", "CAB-LTPC-C3C-POL-1K-001"},
		{"e\u0301", "é", "CAB-LTPC-C3C-POL-001"},
	} {
		model := values.NormalizeModel(tc.input)
		d, err := masterDomain.ItemDerive(c, m, map[string]string{"C": "3C"}, model)
		if err != nil {
			t.Fatal(err)
		}
		sku, err := masterDomain.ItemIssue(d.Prefix, 1)
		if err != nil || model != tc.model || sku != tc.sku {
			t.Fatalf("%q: %q %q %v", tc.input, model, sku, err)
		}
	}
}
func TestItemPythonFingerprintJSON(t *testing.T) {
	for _, tc := range []struct{ model, key string }{
		{"strasse", "7445c0e464faf290958a7084ede93e0e6d2136a5cc930323c9f92a4bf5b72572"},
		{"ab42", "0c7458a7f542403ef04ec350881ebe5f4444a3b8e3637b361311c73125d793e3"},
		{"é & <x>", "7c0fc1c49ec4708d459703e9fd079fa653d897a8640a3b3882be88d1d542d0b5"},
		{"😀", "603263d514a8a8f629c2627d557f352c15c7ba5a28098f79486f0f6029ce6c19"},
	} {
		_, key := (ItemValues{}).Keys(masterDomain.ItemIdentity{Family: "LTPC", UOM: "M", Specifications: map[string]string{"C": "3C"}, Manufacturer: "make", Model: tc.model})
		if key != tc.key {
			t.Fatalf("%q: %s", tc.model, key)
		}
	}
}
func TestItemRetainPublishedFamilyCode(t *testing.T) {
	c := itemConfiguration(masterContracts.Template{Active: true}, masterContracts.CategoryItemContext{Code: "LTPC_REV2", FamilyCode: "LTPC", CategoryCode: "CAB", Active: true})
	if c.Data.FamilyCode != "LTPC" {
		t.Fatal("Revision code replaced engineering family")
	}
}
