package dtos

import (
	"strings"
	"testing"
)

func TestEightCharacterPasswordAcrossEntryPoints(t *testing.T) {
	if _, e := ParseLogin(strings.NewReader(`{"identifier":"fixture","password":"Abcd123!"}`)); e != nil {
		t.Fatal("login rejected eight characters", e)
	}
	if _, e := ParsePassword(strings.NewReader(`{"password":"Abcd123!"}`)); e != nil {
		t.Fatal("password rejected eight characters", e)
	}
	if _, e := ParseAccountPassword(strings.NewReader(`{"current_password":"current","password":"Abcd123!"}`)); e != nil {
		t.Fatal("account rejected eight characters", e)
	}
	if _, e := ParseRecoveryComplete(strings.NewReader(`{"token":"` + strings.Repeat("a", 64) + `","password":"Abcd123!"}`)); e != nil {
		t.Fatal("recovery rejected eight characters", e)
	}
	if _, e := ParseAccountPassword(strings.NewReader(`{"current_password":"current","password":"Abc123!"}`)); e == nil {
		t.Fatal("seven characters accepted")
	}
}
