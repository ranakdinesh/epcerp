package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNoticeIdentityReceiptsAndSource(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	s := Source{EventID: id, EntityID: id, ProjectID: id, ActorID: id, Module: "finance_approvals", EntityType: "financial_decision", Action: "approved", ScopeKind: "project", Visibility: "restricted", Version: 1, At: now}
	n, e := New(id, s, Director{ID: id, UserID: id, Version: 1})
	if e != nil || !n.Matches(s) || n.ReadAt != nil || n.AcknowledgedAt != nil {
		t.Fatal(n, e)
	}
	next, changed, e := Receipt(n, id, 1, "acknowledge", now.Add(time.Hour))
	if e != nil || !changed || next.Version != 2 || next.ReadAt == nil || next.AcknowledgedAt == nil || next.DirectorVersion != 1 {
		t.Fatal(next, e)
	}
	again, changed, e := Receipt(next, id, 2, "read", now.Add(2*time.Hour))
	if e != nil || changed || again.Version != 2 {
		t.Fatal("receipt replay", e)
	}
	if _, _, e = Receipt(next, id, 1, "read", now); !errors.Is(e, Stale) {
		t.Fatal(e)
	}
	if _, _, e = Receipt(n, "22222222-2222-4222-8222-222222222222", 1, "read", now); !errors.Is(e, Missing) {
		t.Fatal(e)
	}
	if _, _, e = Receipt(n, id, 1, "resolved", now); e == nil {
		t.Fatal("notice resolved underlying work")
	}
	if _, _, e = Receipt(n, id, 1, "read", now.Add(-time.Hour)); e == nil {
		t.Fatal("receipt moved backward")
	}
	for _, bad := range []Source{{}, func() Source { x := s; x.Module = "purchase_bo"; return x }(), func() Source { x := s; x.Action = "save"; return x }(), func() Source { x := s; x.Visibility = "public"; return x }()} {
		if bad.Validate() == nil {
			t.Fatal("invalid source", bad)
		}
	}
	if (Actor{ID: id, Active: true, Permissions: []string{"Director"}}).Can("communication.inbox.read") {
		t.Fatal("role name authorized inbox")
	}
	for _, q := range []Query{{Page: 0, Size: 20}, {Page: 1, Size: 1000}, {Page: 1, Size: 20, Status: "resolved"}} {
		if _, e = q.Validate(); e == nil {
			t.Fatal("unbounded or fake status", q)
		}
	}
}

func TestOwnAccountsAcceptanceFinanceNoticeSource(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	s := Source{EventID: id, EntityID: id, ProjectID: id, ActorID: id, Module: "finance_accounts_acceptance", EntityType: "financial_decision", Action: "approved", ScopeKind: "project", Visibility: "restricted", Version: 2, At: time.Now()}
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	s.Action = "returned"
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	s.EntityType = "accounts_acceptance"
	if s.Validate() == nil {
		t.Fatal("pending preparation created notice")
	}
}
