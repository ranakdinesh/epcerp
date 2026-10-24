package application

import (
	"context"
	"errors"
	d "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/ports"
	"testing"
)

type auth struct{ a d.Actor }

func (a auth) Resolve(context.Context, string) (d.Actor, error) { return a.a, nil }

type values struct{}

func (values) Now() d.Timestamp { return d.Timestamp{}.AddDate(2026, 9, 10) }

type reader struct {
	allowed error
	called  bool
}

func (r *reader) Authorize(context.Context, d.Actor) error { return r.allowed }
func (r *reader) Assigned(context.Context, d.Actor, d.Query) ([]d.Task, int64, error) {
	r.called = true
	return []d.Task{{ID: "own", Relation: "doer", PlannedDate: "2020-01-01"}}, 1, nil
}

type unit struct {
	r    *reader
	fail bool
}

func (u unit) Within(c context.Context, f func(context.Context, ports.Reader) error) error {
	if e := f(c, u.r); e != nil {
		return e
	}
	if u.fail {
		return errors.New("commit rejected")
	}
	return nil
}
func TestAuthorityAndCommit(t *testing.T) {
	a := d.Actor{ID: "user", Active: true, Permissions: []string{"work.tasks.view", "work.tasks.read"}}
	r := &reader{}
	s := New(unit{r, false}, auth{a}, values{})
	p, e := s.Page(t.Context(), "token", d.Query{})
	if e != nil || p.Total != 1 || !p.Rows[0].Overdue {
		t.Fatal(p, e)
	}
	r.called = false
	r.allowed = d.Forbidden
	if _, e = s.Page(t.Context(), "token", d.Query{}); e == nil || r.called {
		t.Fatal("live authorization bypass")
	}
	r.allowed = nil
	s = New(unit{r, true}, auth{a}, values{})
	p, e = s.Page(t.Context(), "token", d.Query{})
	if e == nil || len(p.Rows) != 0 || p.Total != 0 {
		t.Fatal("uncommitted projection leaked")
	}
	a.Permissions = nil
	r.called = false
	s = New(unit{r, false}, auth{a}, values{})
	if _, e = s.Page(t.Context(), "token", d.Query{}); e == nil || r.called {
		t.Fatal("missing grant")
	}
	a.Active = false
	if _, e = New(unit{r, false}, auth{a}, values{}).Page(t.Context(), "", d.Query{}); e != d.Unauthorized {
		t.Fatal(e)
	}
}
