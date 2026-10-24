package domain

import (
	"testing"
)

func TestOwnershipRules(t *testing.T) {
	if NameKey(" EPC Project ") != NameKey("epc project") {
		t.Fatal("duplicate normalization")
	}
	p := Principal{ID: "manager", Active: true, Permissions: []string{"projects.master.read"}}
	if !CanRead(p, Project{ManagerID: "manager"}) || CanRead(p, Project{ManagerID: "other"}) {
		t.Fatal("scope")
	}
	p.Permissions = append(p.Permissions, "projects.portfolio.read")
	if !CanRead(p, Project{ManagerID: "other"}) {
		t.Fatal("leadership")
	}
	p.Active = false
	if CanRead(p, Project{ManagerID: "manager"}) {
		t.Fatal("inactive identity")
	}
	c := Candidate{Active: true, Permissions: []string{"projects.master.read"}}
	if c.Eligible() {
		t.Fatal("read-only candidate")
	}
	c.Permissions = append(c.Permissions, "projects.master.write")
	if !c.Eligible() {
		t.Fatal("eligible candidate")
	}
	if _, e := (Transfer{ManagerID: "00000000-0000-0000-0000-000000000001", Reason: " ", ExpectedVersion: 1}).Validate(); e == nil {
		t.Fatal("missing reason")
	}
	if _, e := (Filter{Page: 1, PageSize: 101}).Validate(); e == nil {
		t.Fatal("unbounded page")
	}
}
