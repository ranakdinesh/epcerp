package domain

import (
	"github.com/ranakdinesh/epcERP/internal/identity"
	"strings"
	"testing"
)

func TestNamedTenderValidationAndLeastPrivilege(t *testing.T) {
	id := "18e66d87-c4db-4771-bbdb-03ad3684e345"
	for _, name := range []string{"", "x", strings.Repeat("अ", 256)} {
		if _, e := (NamedInput{Name: name, ManagerID: id}).Normalize(); e == nil {
			t.Fatal("invalid name accepted")
		}
	}
	in, e := (NamedInput{Name: "  Solar EPC opportunity  ", ManagerID: strings.ToUpper(id)}).Normalize()
	if e != nil || in.Name != "Solar EPC opportunity" || in.ManagerID != id {
		t.Fatal(in, e)
	}
	p := identity.Principal{ID: id, Active: true, Permissions: ManagerPermissions()}
	u := EligibleIdentity{Principal: p, Ready: true, Designated: true}
	if !u.CanManage() || u.Principal.Allows("projects.budgets.approve") {
		t.Fatal("profile authority")
	}
	for _, which := range []string{"designation", "ready", "active", "permission"} {
		v := u
		switch which {
		case "designation":
			v.Designated = false
		case "ready":
			v.Ready = false
		case "active":
			v.Principal.Active = false
		case "permission":
			v.Principal.Permissions = v.Principal.Permissions[:3]
		}
		if v.CanManage() {
			t.Fatal("ineligible accepted", which)
		}
	}
	u.Principal.Superuser = true
	u.Designated = false
	if u.CanManage() {
		t.Fatal("recovery administrator silently designated")
	}
	v := Record{SalesCreatorID: "sales", ManagerID: id}
	if !v.CanRead(p) {
		t.Fatal("manager scope")
	}
	p.ID = "outsider"
	if v.CanRead(p) {
		t.Fatal("permission alone grants cross tender")
	}
	p.ID = "sales"
	if !v.CanRead(p) {
		t.Fatal("creator scope")
	}
}
