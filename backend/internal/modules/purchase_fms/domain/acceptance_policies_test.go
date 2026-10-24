package domain

import (
	"testing"
	"time"
)

const acceptanceFirst = "11111111-1111-4111-8111-111111111111"
const acceptanceSecond = "22222222-2222-4222-8222-222222222222"
const acceptanceThird = "33333333-3333-4333-8333-333333333333"

func acceptanceFixture() (Principal, AcceptanceAccess, AcceptanceSource, AcceptanceCommand, AcceptanceTimestamp) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	return Principal{ID: acceptanceFirst, Active: true}, AcceptanceAccess{Read: true, Write: true, Work: true, Scoped: true}, AcceptanceSource{ProjectID: acceptanceFirst, PackageID: acceptanceSecond, Available: true, Order: &AcceptanceOrder{ID: acceptanceThird, ProjectID: acceptanceFirst, PackageID: acceptanceSecond, Version: 2, Number: "PO/VTP/2026-27/000001", Date: "2026-10-10", SupplierID: acceptanceThird, SupplierName: "Actual vendor", Quantity: "10", UOM: "metre", Currency: "INR", Total: "145.69", ReleasedAt: now.Add(-24 * time.Hour), ReleasedBy: acceptanceFirst}}, AcceptanceCommand{Action: "record", Reason: "Vendor response received", Response: AcceptanceResponse{Status: "accepted", AcceptedDate: "2026-10-11"}}, now
}
func TestAcceptanceAcceptanceRetainsPythonFieldsAndOwnedOrder(t *testing.T) {
	a, c, s, q, n := acceptanceFixture()
	q.Response.SignatoryName = "Vendor authorised person"
	q.Response.DocumentReference = "Vendor email reference"
	q.Response.Remarks = "Confirmed"
	r, e := AcceptanceApply(nil, a, c, q, s, n, acceptanceThird)
	if e != nil {
		t.Fatal(e)
	}
	if r.Order != *s.Order || r.Response != q.Response || r.Version != 1 || r.CreatedBy != a.ID || r.UpdatedBy != a.ID {
		t.Fatal("lost source/response/audit")
	}
	fact, e := AcceptanceAcceptedFact(&r, s)
	if e != nil || fact == nil {
		t.Fatal(e)
	}
	fact.Response.Status = "pending"
	if r.Response.Status != "accepted" {
		t.Fatal("published fact aliased")
	}
	q.Version = 1
	q.Response.Status = "accepted_with_exceptions"
	q.Response.Remarks = "Delivery exception"
	revised, e := AcceptanceApply(&r, a, c, q, s, n.Add(time.Minute), acceptanceFirst)
	if e != nil {
		t.Fatal(e)
	}
	if revised.ID != r.ID || revised.CreatedAt != r.CreatedAt || revised.CreatedBy != r.CreatedBy || revised.Version != 2 {
		t.Fatal("identity mutated")
	}
	if _, e = AcceptanceAcceptedFact(&revised, s); e == nil {
		t.Fatal("exceptions granted clearance")
	}
	q.Version = 2
	q.Response.Status = "pending"
	q.Response.AcceptedDate = ""
	q.Response.Remarks = "Awaiting unconditional response"
	pending, e := AcceptanceApply(&revised, a, c, q, s, n, acceptanceFirst)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = AcceptanceAcceptedFact(&pending, s); e == nil {
		t.Fatal("pending granted clearance")
	}
}
func TestAcceptanceAcceptanceRejectsInvalidOrForeignEvidence(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*Principal, *AcceptanceAccess, *AcceptanceSource, *AcceptanceCommand)
	}{
		{"missing read", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) { c.Read = false }},
		{"missing write", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) { c.Write = false }},
		{"not doer", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) { c.Work = false }},
		{"inactive", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) { a.Active = false }},
		{"held", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			s.Available = false
		}},
		{"blocker", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			s.Blockers = []string{"Unresolved issue"}
		}},
		{"no PO", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) { s.Order = nil }},
		{"draft PO", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			s.Order.Version = 1
		}},
		{"foreign project", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			s.Order.ProjectID = acceptanceThird
		}},
		{"foreign package", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			s.Order.PackageID = acceptanceThird
		}},
		{"missing date", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			q.Response.AcceptedDate = ""
		}},
		{"invalid date", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			q.Response.AcceptedDate = "2026-02-30"
		}},
		{"before PO", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			q.Response.AcceptedDate = "2026-10-09"
		}},
		{"future", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			q.Response.AcceptedDate = "2026-10-12"
		}},
		{"pending date", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			q.Response.Status = "pending"
		}},
		{"exceptions without note", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			q.Response.Status = "accepted_with_exceptions"
		}},
		{"unknown status", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			q.Response.Status = "rejected"
		}},
		{"unknown action", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) {
			q.Action = "approve"
		}},
		{"missing reason", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) { q.Reason = "  " }},
		{"stale create", func(a *Principal, c *AcceptanceAccess, s *AcceptanceSource, q *AcceptanceCommand) { q.Version = 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			a, c, s, q, n := acceptanceFixture()
			test.edit(&a, &c, &s, &q)
			if r, e := AcceptanceApply(nil, a, c, q, s, n, acceptanceThird); e == nil || r.ID != "" {
				t.Fatal("invalid response was published")
			}
		})
	}
}
func TestAcceptanceAcceptanceCorrectionRejectsStaleSource(t *testing.T) {
	a, c, s, q, n := acceptanceFixture()
	r, e := AcceptanceApply(nil, a, c, q, s, n, acceptanceThird)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = AcceptanceApply(&r, a, c, q, s, n, acceptanceFirst); e != AcceptanceStale {
		t.Fatalf("stale version %v", e)
	}
	q.Version = 1
	s.Order.Number = "Different PO"
	if _, e = AcceptanceApply(&r, a, c, q, s, n, acceptanceFirst); e != AcceptanceStale {
		t.Fatalf("changed source %v", e)
	}
	if _, e = AcceptanceAcceptedFact(&r, s); e != AcceptanceStale {
		t.Fatalf("fact accepted changed PO %v", e)
	}
}
