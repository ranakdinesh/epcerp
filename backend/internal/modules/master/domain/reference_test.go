package domain

import (
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
	"strings"
	"testing"
)

func TestReferenceNormalizationAndLinks(t *testing.T) {
	if ReferenceNormalize("  Straße  Σς  ") != "straße σσ" || ReferenceCode(" Straße & Company ") != "STRA-E-COMPANY" {
		t.Fatal("Unicode normalization")
	}
	if len(ReferenceCode(strings.Repeat("AB", 50))) != 32 {
		t.Fatal("code length")
	}
	for _, test := range []struct {
		kind  string
		input ReferenceInput
	}{{"cities", ReferenceInput{Name: "City"}}, {"states", ReferenceInput{Name: "State", StateID: "parent"}}, {"industries", ReferenceInput{Name: "Industry", Code: "---"}}} {
		if _, e := ReferencePrepare(test.kind, test.input); e == nil {
			t.Fatal("invalid master accepted")
		}
	}
	in, e := ReferencePrepare("client-types", ReferenceInput{Name: "  Public   Sector "})
	if e != nil || in.Name != "Public Sector" || in.Code != "PUBLIC-SECTOR" {
		t.Fatal(in, e)
	}
}
func TestReferenceResourcePermissions(t *testing.T) {
	for k, r := range map[string]string{"client-types": "client_types", "industries": "industries", "states": "locations", "cities": "locations"} {
		if ReferencePermission(k, true) != "settings."+r+".write" || ReferencePermission(k, false) != "settings."+r+".read" {
			t.Fatal(k)
		}
	}
	if (sharedIdentity.Principal{Active: true, Permissions: []string{"system:admin"}}).Allows("settings.locations.write") {
		t.Fatal("ordinary recovery string bypass")
	}
}
