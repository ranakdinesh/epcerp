package domain

import (
	"errors"
	"testing"
	"time"
)

func TestDirectorLifecyclePreservesIdentityAndAttribution(t *testing.T) {
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	id := "00000000-0000-4000-8000-000000000001"
	a := "00000000-0000-4000-8000-000000000002"
	b := "00000000-0000-4000-8000-000000000003"
	first, action, e := Change(nil, Input{UserID: a, Active: true}, id, a, now)
	if e != nil || action != "designate" || first.Version != 1 {
		t.Fatal(first, action, e)
	}
	for _, tc := range []struct {
		input  Input
		action string
	}{{Input{UserID: b, Active: true, Version: 1}, "change"}, {Input{UserID: b, Active: false, Version: 2}, "revoke"}, {Input{UserID: b, Active: true, Version: 3}, "reactivate"}} {
		next, action, e := Change(&first, tc.input, "ignored", b, now.Add(time.Hour))
		if e != nil || action != tc.action || next.ID != id || next.CreatedAt != now || *next.CreatedBy != a || *next.UpdatedBy != b {
			t.Fatal(next, action, e)
		}
		first = next
	}
	if _, _, e = Change(&first, Input{UserID: b, Active: true, Version: 0}, id, a, now); !errors.Is(e, Stale) {
		t.Fatal("stale accepted", e)
	}
	if _, _, e = Change(&first, Input{UserID: b, Active: true, Version: first.Version}, id, a, now); e == nil {
		t.Fatal("no-op accepted")
	}
	if _, _, e = Change(nil, Input{UserID: b, Active: false}, id, a, now); e == nil {
		t.Fatal("empty revocation accepted")
	}
}
func TestDirectorInputAndActorPermissions(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	valid := Input{UserID: id, Reason: " Initial designation ", Active: true}
	i, e := valid.Validate()
	if e != nil || i.Reason != "Initial designation" {
		t.Fatal(e)
	}
	for _, v := range []Input{{UserID: "not-id", Reason: "Reason"}, {UserID: id, Reason: " "}, {UserID: id, Reason: "Reason", Version: -1}} {
		if _, e = v.Validate(); e == nil {
			t.Fatal("invalid accepted")
		}
	}
	for _, a := range []Actor{{ID: id, Active: false, Superuser: true}, {ID: id, Active: true, Permissions: []string{"Director"}}, {ID: id, Active: true, Permissions: []string{"projects.team.write"}}} {
		if a.Can("admin.governance.write") {
			t.Fatal("inferred authority")
		}
	}
}
