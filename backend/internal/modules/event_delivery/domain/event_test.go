package domain

import (
	"testing"
	"time"
)

func validEvent() Event {
	return Event{ID: "00000000-0000-4000-8000-000000000001", Module: "projects", EntityType: "project", EntityID: "00000000-0000-4000-8000-000000000002", EntityVersion: 1, ActorKind: "system", Action: "created", CorrelationID: "00000000-0000-4000-8000-000000000003", ScopeKind: "platform", Visibility: "restricted", OccurredAt: time.Date(2026, 10, 9, 12, 0, 0, 123456789, time.FixedZone("IST", 19800))}
}
func TestEventMetadata(t *testing.T) {
	e := validEvent()
	clean, err := e.Normalize()
	if err != nil || clean.OccurredAt.Location() != time.UTC || clean.OccurredAt.Nanosecond() != 123456000 {
		t.Fatal("timestamp normalization", err)
	}
	cases := []func(*Event){func(e *Event) { e.ID = "invalid" }, func(e *Event) { e.Module = "personal name" }, func(e *Event) { e.Action = "password=value" }, func(e *Event) { e.EntityVersion = 0 }, func(e *Event) { e.ScopeKind = "project" }, func(e *Event) { e.ScopeID = e.ID }, func(e *Event) { e.ActorID = "name" }, func(e *Event) { e.ActorKind = "user" }, func(e *Event) { e.ActorKind = "" }, func(e *Event) { e.CausationID = "name" }, func(e *Event) { e.Visibility = "public" }, func(e *Event) { e.OccurredAt = time.Time{} }}
	for i, change := range cases {
		e := validEvent()
		change(&e)
		if _, err := e.Normalize(); err != ErrInvalid {
			t.Fatalf("invalid metadata %d accepted", i)
		}
	}
	e = validEvent()
	e.ScopeKind = "project"
	e.ScopeID = e.EntityID
	if _, err := e.Normalize(); err != nil {
		t.Fatal(err)
	}
}
func TestDeliveryPolicy(t *testing.T) {
	p := Policy{LeaseSeconds: 30, HandlerSeconds: 20, RetrySeconds: 2, MaxRetrySeconds: 10}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	for attempt, want := range []int32{2, 4, 8, 10, 10} {
		if got := p.RetryDelay(int32(attempt + 1)); got != want {
			t.Fatal(got, want)
		}
	}
	p.HandlerSeconds = p.LeaseSeconds
	if p.Validate() != ErrInvalid {
		t.Fatal("unbounded handler accepted")
	}
	for _, targets := range [][]Target{{{Consumer: "inbox", MaxAttempts: 0}}, {{Consumer: "mail@example.invalid", MaxAttempts: 1}}, {{Consumer: "inbox", MaxAttempts: 2}, {Consumer: "inbox", MaxAttempts: 2}}} {
		if ValidateTargets(targets) != ErrInvalid {
			t.Fatal("unsafe target accepted")
		}
	}
}
