package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type advancesQueueFixture struct {
	advancesFixture
	labelCalls int
	repoCalls  int
	rows       []domain.AdvancesRecord
}

func (f *advancesQueueFixture) Queue(context.Context, int32) ([]domain.AdvancesRecord, error) {
	f.repoCalls++
	return f.rows, nil
}
func (f *advancesQueueFixture) ProjectLabel(_ context.Context, p string) (string, string, error) {
	f.labelCalls++
	if p != advancesProject {
		return "", "", errors.New("unauthorized project label requested")
	}
	return "Assigned project", "PRJ-1", nil
}
func (f *advancesQueueFixture) FinanceRead(_ context.Context, _ domain.Principal, p string) (bool, error) {
	return p == advancesProject, nil
}
func (f *advancesQueueFixture) Within(ctx context.Context, work func(context.Context, ports.AdvancesRepository, ports.AdvancesContext, ports.AdvancesFinance) error) error {
	e := work(ctx, f, f, f)
	if e == nil && f.commitFails {
		return errors.New("fixture commit failed")
	}
	return e
}
func (f *advancesQueueFixture) Resolve(_ context.Context, token string) (domain.Principal, error) {
	return domain.Principal{ID: advancesReviewer, Active: token != "", Permissions: []string{"accounts.purchase_advances.view", "accounts.purchase_advances.read"}}, nil
}
func TestAdvancesQueueFiltersBeforeReadingProjectLabelsAndClearsFailedCommit(t *testing.T) {
	f := &advancesQueueFixture{rows: []domain.AdvancesRecord{{ProjectID: advancesProject, PackageID: advancesPkg, Version: 1}, {ProjectID: advancesIdentity, PackageID: advancesProject, Version: 2}}}
	s := NewAdvances(f, f, f)
	out, e := s.Queue(t.Context(), "accounts", 0)
	if e != nil || len(out.Rows) != 1 || out.Rows[0].ProjectID != advancesProject || f.labelCalls != 1 {
		t.Fatal("queue scope leak", out, e)
	}
	f.commitFails = true
	out, e = s.Queue(t.Context(), "accounts", 0)
	if e == nil || len(out.Rows) != 0 {
		t.Fatal("failed transaction exposed rows")
	}
	calls := f.repoCalls
	if _, e = s.Queue(t.Context(), "accounts", -1); e == nil || f.repoCalls != calls {
		t.Fatal("invalid offset reached persistence")
	}
	if _, e = s.Queue(t.Context(), "", 0); e == nil || f.repoCalls != calls {
		t.Fatal("anonymous queue reached persistence")
	}
}
