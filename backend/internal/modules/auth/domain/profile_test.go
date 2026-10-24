package domain

import (
	"strings"
	"testing"

	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
)

func TestProfilePolicy(t *testing.T) {
	if v, e := ProfileName("  Staff Member  "); e != nil || v != "Staff Member" {
		t.Fatal("name normalization")
	}
	for _, v := range []string{strings.Repeat("x", 101), "unsafe\nname"} {
		if _, e := ProfileName(v); e == nil {
			t.Fatal("invalid name accepted")
		}
	}
	p := sharedIdentity.Principal{ID: "actor", Active: true, Permissions: []string{"account.profile.read"}}
	if !p.Allows("account.profile.read") || p.Allows("account.profile.write") {
		t.Fatal("permission leak")
	}
	p.Active = false
	p.Superuser = true
	if p.Allows("account.profile.read") {
		t.Fatal("inactive allowed")
	}
}
