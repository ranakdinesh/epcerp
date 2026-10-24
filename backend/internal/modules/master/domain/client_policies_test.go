package domain

import (
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
	"testing"

	"github.com/ranakdinesh/epcERP/helpers"
)

func TestClientTaxAndNormalization(t *testing.T) {
	if helpers.Lower("İ Straße") != "i straße" {
		t.Fatal("native lowercase")
	}
	if ClientNormalize("  Straße   Σς  ") != "straße σσ" {
		t.Fatal("casefold")
	}
	if !ClientValidGST("27AAACW4202F1ZQ") || ClientValidGST("27AAACW4202F1Z3") || ClientValidPAN("AAACW0000F") {
		t.Fatal("tax")
	}
	if ClientValidateState("27AAACW4202F1ZQ", "Maharashtra") != nil || ClientValidateState("27AAACW4202F1ZQ", "Punjab") == nil {
		t.Fatal("state")
	}
	var c Client
	if ClientPrepare(ClientInput{Contacts: []ClientContact{{Primary: true}, {Primary: true}}}, &c) == nil {
		t.Fatal("ambiguous primary")
	}
}
func TestClientIDAndPermission(t *testing.T) {
	id, e := ClientCanonicalID("{AABBCCDD-0000-4000-8000-AABBCCDDEEFF}")
	if e != nil || id != "aabbccdd-0000-4000-8000-aabbccddeeff" {
		t.Fatal(id, e)
	}
	if (sharedIdentity.Principal{Active: false, Superuser: true}).Allows("settings.clients.write") || (sharedIdentity.Principal{Active: true, Permissions: []string{"system:admin"}}).Allows("settings.clients.write") {
		t.Fatal("permission bypass")
	}
}
