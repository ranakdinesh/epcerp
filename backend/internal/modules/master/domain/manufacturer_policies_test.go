package domain

import (
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
	"testing"
)

func TestManufacturerIdentityAndEligibility(t *testing.T) {
	if code, e := ManufacturerValidateCode("ß"); e == nil || code != "" {
		t.Fatal("non-ASCII code must be rejected", code, e)
	}
	if ManufacturerNameKey(" Straße ") != "straße" || ManufacturerNameKey("Two  Spaces") == ManufacturerNameKey("Two Spaces") {
		t.Fatal("name normalization changed")
	}
	if s, e := ManufacturerValidateCode("ab1"); e != nil || s != "AB1" {
		t.Fatal(s, e)
	}
	for _, data := range []ManufacturerData{{}, {Eligible: []string{"A", "A"}}, {Eligible: []string{"a"}}, {Eligible: []string{"inactive"}}} {
		if ManufacturerValidateEligibility(data, []string{"A"}) == nil {
			t.Fatal("invalid eligibility accepted")
		}
	}
	if ManufacturerValidateEligibility(ManufacturerData{Eligible: []string{"A"}}, []string{"A"}) != nil {
		t.Fatal("active eligibility rejected")
	}
}
func TestManufacturerRecoveryPermission(t *testing.T) {
	if (sharedIdentity.Principal{Active: true, Permissions: []string{"system:admin"}}).Allows("settings.manufacturers.write") || (sharedIdentity.Principal{Active: false, Superuser: true}).Allows("settings.manufacturers.read") {
		t.Fatal("bypass")
	}
}
