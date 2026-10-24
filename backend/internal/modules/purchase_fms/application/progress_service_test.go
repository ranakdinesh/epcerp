package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const progressProject = "11111111-1111-4111-8111-111111111111"

type progressAuth struct{ a domain.Principal }

func (a progressAuth) Resolve(context.Context, string) (domain.Principal, error) { return a.a, nil }

type progressClock struct{}

func (progressClock) Now() domain.ProgressTimestamp {
	return domain.ProgressTimestamp{}.AddDate(2026, 9, 10)
}

type progressReader struct {
	deny   error
	called bool
	source domain.ProgressSource
}

func (r *progressReader) Authorize(context.Context, domain.Principal, string) error { return r.deny }
func (r *progressReader) Source(context.Context, string) (domain.ProgressSource, error) {
	r.called = true
	return r.source, nil
}

type progressUnit struct {
	r    *progressReader
	fail bool
}

func (u progressUnit) Within(ctx context.Context, f func(context.Context, ports.ProgressReader) error) error {
	if e := f(ctx, u.r); e != nil {
		return e
	}
	if u.fail {
		return errors.New("commit failure")
	}
	return nil
}
func TestProgressLiveProjectAuthorityBeforeEvidence(t *testing.T) {
	a := domain.Principal{ID: "actor", Active: true, Permissions: []string{"projects.purchase_progress.view", "projects.purchase_progress.read"}}
	r := &progressReader{source: domain.ProgressSource{ProjectID: progressProject}}
	s := NewProgress(progressUnit{r, false}, progressAuth{a}, progressClock{})
	report, e := s.Report(t.Context(), "token", progressProject)
	if e != nil || report.Progress != nil || report.Packages == nil {
		t.Fatal(report, e)
	}
	r.called = false
	r.deny = domain.ProgressForbidden
	if _, e = s.Report(t.Context(), "token", progressProject); e != domain.ProgressForbidden || r.called {
		t.Fatal("scoped evidence leak", e)
	}
	r.deny = nil
	r.source.ProjectID = "other"
	if report, e = s.Report(t.Context(), "token", progressProject); e == nil || report.ProjectID != "" {
		t.Fatal("cross-project source leak")
	}
	r.source.ProjectID = progressProject
	s = NewProgress(progressUnit{r, true}, progressAuth{a}, progressClock{})
	if report, e = s.Report(t.Context(), "token", progressProject); e == nil || report.ProjectID != "" || report.Packages != nil {
		t.Fatal("uncommitted projection leaked")
	}
	a.Permissions = nil
	r.called = false
	if _, e = NewProgress(progressUnit{r, false}, progressAuth{a}, progressClock{}).Report(t.Context(), "token", progressProject); e != domain.ProgressForbidden || r.called {
		t.Fatal("global grant bypass")
	}
	a.Active = false
	if _, e = NewProgress(progressUnit{r, false}, progressAuth{a}, progressClock{}).Report(t.Context(), "token", progressProject); e != domain.ProgressUnauthorized {
		t.Fatal("inactive actor")
	}
}

type progressRevokedReader struct{ calls int }

func (r *progressRevokedReader) Authorize(context.Context, domain.Principal, string) error {
	r.calls++
	if r.calls == 2 {
		return domain.ProgressForbidden
	}
	return nil
}
func (r *progressRevokedReader) Source(context.Context, string) (domain.ProgressSource, error) {
	return domain.ProgressSource{ProjectID: progressProject}, nil
}

type progressRevokedUnit struct{ r *progressRevokedReader }

func (u progressRevokedUnit) Within(ctx context.Context, f func(context.Context, ports.ProgressReader) error) error {
	return f(ctx, u.r)
}
func TestProgressRecheckAfterSourceCollection(t *testing.T) {
	a := domain.Principal{ID: "actor", Active: true, Permissions: []string{"projects.purchase_progress.view", "projects.purchase_progress.read"}}
	r := &progressRevokedReader{}
	report, e := NewProgress(progressRevokedUnit{r}, progressAuth{a}, progressClock{}).Report(t.Context(), "token", progressProject)
	if e != domain.ProgressForbidden || report.ProjectID != "" || r.calls != 2 {
		t.Fatal("read-time revocation leaked report", report, e)
	}
}
