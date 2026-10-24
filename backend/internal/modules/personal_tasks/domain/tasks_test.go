package domain

import (
	"strings"
	"testing"
	"time"
)

func TestQueryBounds(t *testing.T) {
	q, e := (Query{}).Validate()
	if e != nil || q.Page != 1 || q.Size != 20 || q.Role != "all" {
		t.Fatal(q, e)
	}
	for _, q := range []Query{{Page: -1}, {Page: 100001}, {Size: 21}, {Role: "manager"}, {State: "complete"}, {Search: strings.Repeat("界", 121)}} {
		if _, e := q.Validate(); e == nil {
			t.Fatal("unbounded query", q)
		}
	}
	if _, e := (Query{Search: strings.Repeat("界", 120), Size: 100}).Validate(); e != nil {
		t.Fatal(e)
	}
}
func TestIndiaDeadlineAndResolver(t *testing.T) {
	now := time.Date(2026, 10, 10, 19, 0, 0, 0, time.UTC)
	task := Task{Relation: "doer", PlannedDate: "2026-10-10", ResolutionDue: "2026-10-11", Blockers: []string{"held"}}
	if !task.WithClock(now).Overdue {
		t.Fatal("India date rollover")
	}
	task.Relation = "resolver"
	if task.WithClock(now).Overdue {
		t.Fatal("resolver used doer date")
	}
	copy := task.WithClock(now)
	copy.Blockers[0] = "changed"
	if task.Blockers[0] != "held" {
		t.Fatal("aliased blockers")
	}
	task.ResolutionDue = ""
	if task.WithClock(now).Overdue {
		t.Fatal("missing deadline")
	}
}
