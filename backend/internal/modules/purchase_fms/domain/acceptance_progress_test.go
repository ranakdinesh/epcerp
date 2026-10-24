package domain

import (
	"testing"
)

func TestAcceptanceProgressDoesNotTurnExceptionsOrHoldsIntoAuthority(t *testing.T) {
	a, c, s, q, n := acceptanceFixture()
	r, e := AcceptanceApply(nil, a, c, q, s, n, acceptanceThird)
	if e != nil {
		t.Fatal(e)
	}
	s.Available = false
	s.Blockers = []string{"Held project"}
	p := AcceptanceProgressFact(&r, s)
	if p.State != "completed" || !p.Current || !p.WithoutExceptions || p.Snapshot == nil {
		t.Fatal("held acknowledgement erased", p)
	}
	if _, e = AcceptanceAcceptedFact(&r, s); e == nil {
		t.Fatal("reporting removed hold gate")
	}
	r.Response.Status = "accepted_with_exceptions"
	r.Response.Remarks = "Delivery date exception"
	p = AcceptanceProgressFact(&r, s)
	if p.State != "completed" || p.WithoutExceptions {
		t.Fatal("exception granted downstream readiness", p)
	}
	p.Snapshot.Response.Remarks = "changed"
	if r.Response.Remarks != "Delivery date exception" {
		t.Fatal("aliased reporting snapshot")
	}
	s.Order.Version++
	p = AcceptanceProgressFact(&r, s)
	if p.Current || p.State != "blocked" {
		t.Fatal("obsolete PO acknowledgement credited", p)
	}
}

func TestAcceptanceProgressRequiresActualCurrentAcknowledgement(t *testing.T) {
	a, access, source, command, now := acceptanceFixture()
	record, err := AcceptanceApply(nil, a, access, command, source, now, acceptanceThird)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		edit    func(*AcceptanceRecord, *AcceptanceSource)
		want    string
		current bool
	}{
		{"inactive", func(r *AcceptanceRecord, s *AcceptanceSource) { r.Active = false }, "not_started", false},
		{"pending", func(r *AcceptanceRecord, s *AcceptanceSource) { r.Response.Status = "pending" }, "not_started", true},
		{"missing order", func(r *AcceptanceRecord, s *AcceptanceSource) { s.Order = nil }, "blocked", false},
		{"foreign package", func(r *AcceptanceRecord, s *AcceptanceSource) { s.PackageID = acceptanceFirst }, "blocked", false},
		{"unknown response", func(r *AcceptanceRecord, s *AcceptanceSource) { r.Response.Status = "completed" }, "blocked", true},
		{"missing actual date", func(r *AcceptanceRecord, s *AcceptanceSource) { r.Response.AcceptedDate = "" }, "blocked", true},
		{"date before order", func(r *AcceptanceRecord, s *AcceptanceSource) { r.Response.AcceptedDate = "2000-01-01" }, "blocked", true},
		{"unexplained exception", func(r *AcceptanceRecord, s *AcceptanceSource) {
			r.Response.Status = "accepted_with_exceptions"
			r.Response.Remarks = ""
		}, "blocked", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := record
			s := source
			tc.edit(&r, &s)
			p := AcceptanceProgressFact(&r, s)
			if p.State != tc.want || p.Current != tc.current || p.WithoutExceptions || p.Snapshot != nil {
				t.Fatal("invalid acknowledgement earned completion", p)
			}
		})
	}
	if p := AcceptanceProgressFact(nil, source); p.State != "not_started" || p.Current || p.Snapshot != nil {
		t.Fatal(p)
	}
}
