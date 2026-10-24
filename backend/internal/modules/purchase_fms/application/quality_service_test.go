package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

const qualityProject = "11111111-1111-4111-8111-111111111111"
const qualityPkg = "22222222-2222-4222-8222-222222222222"
const qualityActor = "33333333-3333-4333-8333-333333333333"
const qualityRecordID = "44444444-4444-4444-8444-444444444444"

type qualityFixture struct {
	record                  *domain.QualityRecord
	history                 []domain.QualityEvent
	caps                    domain.QualityAccess
	source                  domain.QualitySource
	commitFails, eventFails bool
	sourceCalls             int
}

func (f *qualityFixture) Within(ctx context.Context, fn func(context.Context, ports.QualityRepository, ports.QualityContext) error) error {
	var old *domain.QualityRecord
	if f.record != nil {
		v := domain.QualityCopy(*f.record)
		old = &v
	}
	hist := append([]domain.QualityEvent{}, f.history...)
	e := fn(ctx, f, f)
	if e == nil && f.commitFails {
		e = errors.New("late commit failure")
	}
	if e != nil {
		f.record = old
		f.history = hist
	}
	return e
}
func (f *qualityFixture) Get(context.Context, string, string) (*domain.QualityRecord, error) {
	if f.record == nil {
		return nil, nil
	}
	v := domain.QualityCopy(*f.record)
	return &v, nil
}
func (f *qualityFixture) Save(_ context.Context, v domain.QualityRecord) (domain.QualityRecord, error) {
	c := domain.QualityCopy(v)
	f.record = &c
	return domain.QualityCopy(c), nil
}
func (f *qualityFixture) Event(_ context.Context, v domain.QualityEvent) error {
	if f.eventFails {
		return errors.New("late journal failure")
	}
	f.history = append(f.history, v)
	return nil
}
func (f *qualityFixture) History(context.Context, string) ([]domain.QualityEvent, error) {
	return append([]domain.QualityEvent{}, f.history...), nil
}
func (f *qualityFixture) Access(context.Context, domain.Principal, string, string) (domain.QualityAccess, error) {
	return f.caps, nil
}
func (f *qualityFixture) Source(context.Context, string, string) (domain.QualitySource, error) {
	f.sourceCalls++
	return f.source, nil
}
func (f *qualityFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: qualityActor, Active: true}, nil
}
func (f *qualityFixture) ID() string { return qualityRecordID }
func (f *qualityFixture) Now() domain.QualityTimestamp {
	return domain.QualityTimestamp{}.AddDate(2025, 9, 10)
}
func qualitySetup() *qualityFixture {
	return &qualityFixture{caps: domain.QualityAccess{Read: true, Write: true, Verify: true, Scoped: true, Work: true}, source: domain.QualitySource{Available: true, Blockers: []string{}, QualitySourceFacts: domain.QualitySourceFacts{ProjectID: qualityProject, PackageID: qualityPkg, ReportID: qualityRecordID, ReportVersion: 6, ReportRevision: 3, IRNumber: "IR-1", IRNNumber: "IRN-1", IssueDate: "2026-10-10", OrderID: qualityRecordID, OrderVersion: 2, Number: "PO-1", SupplierName: "Supplier", Description: "Inspected material", Quantity: "1.000001", UOM: "metre", InspectionID: qualityRecordID, InspectionVersion: 8}}}
}
func qualityCommand() domain.QualityCommand {
	return domain.QualityCommand{Action: "record", Code: "TC", Reason: "Record actual certificate", Details: &domain.QualityDetails{Number: "TC-1", Issuer: "Test lab"}}
}
func TestQualityQualityCommitAndJournalFailuresRetainPriorTruth(t *testing.T) {
	f := qualitySetup()
	s := NewQuality(f, f, f)
	for _, failure := range []string{"journal", "commit"} {
		f.eventFails = failure == "journal"
		f.commitFails = failure == "commit"
		w, e := s.Execute(t.Context(), "actor", qualityProject, qualityPkg, qualityCommand())
		if e == nil || w.Record != nil || f.record != nil || len(f.history) != 0 {
			t.Fatal("failed write leaked", failure, e)
		}
	}
	f.eventFails = false
	f.commitFails = false
	w, e := s.Execute(t.Context(), "actor", qualityProject, qualityPkg, qualityCommand())
	if e != nil || w.Record == nil || len(w.Record.Certificates) != 5 || len(w.History) != 1 {
		t.Fatal(e)
	}
	f.commitFails = true
	w, e = s.Execute(t.Context(), "actor", qualityProject, qualityPkg, domain.QualityCommand{Version: 1, Action: "verify", Code: "TC", Reason: "Verify reviewed evidence"})
	if e == nil || w.Record != nil || f.record.Certificates[0].Verified || len(f.history) != 1 {
		t.Fatal("failed verification escaped", e)
	}
}
func TestQualityQualityAuthorizationVersionAndScopePrecedeEvidence(t *testing.T) {
	f := qualitySetup()
	s := NewQuality(f, f, f)
	f.caps.Read = false
	if _, e := s.Workspace(t.Context(), "actor", qualityProject, qualityPkg); e != domain.QualityForbidden || f.sourceCalls != 0 {
		t.Fatal("unauthorized evidence", e)
	}
	f.caps.Read = true
	f.caps.Work = false
	if _, e := s.Execute(t.Context(), "actor", qualityProject, qualityPkg, qualityCommand()); e != domain.QualityForbidden || f.sourceCalls != 0 {
		t.Fatal("unassigned write", e)
	}
	f.caps.Work = true
	q := qualityCommand()
	q.Version = 2
	if _, e := s.Execute(t.Context(), "actor", qualityProject, qualityPkg, q); e != domain.QualityStale || f.sourceCalls != 0 {
		t.Fatal("stale version fetched evidence", e)
	}
	f.source.ProjectID = qualityRecordID
	if w, e := s.Workspace(t.Context(), "actor", qualityProject, qualityPkg); e == nil || w.Record != nil {
		t.Fatal("foreign source leaked")
	}
}
func TestQualityQualitySeparateVerificationPermissionAndTaskRedaction(t *testing.T) {
	f := qualitySetup()
	s := NewQuality(f, f, f)
	if _, e := s.Execute(t.Context(), "actor", qualityProject, qualityPkg, qualityCommand()); e != nil {
		t.Fatal(e)
	}
	f.caps.Verify = false
	q := domain.QualityCommand{Version: 1, Action: "verify", Code: "TC", Reason: "Review actual certificate"}
	if _, e := s.Execute(t.Context(), "actor", qualityProject, qualityPkg, q); e != domain.QualityForbidden {
		t.Fatal("write granted verify", e)
	}
	f.caps.Write = false
	f.caps.Verify = true
	w, e := s.Execute(t.Context(), "actor", qualityProject, qualityPkg, q)
	if e != nil || !w.Record.Certificates[0].Verified {
		t.Fatal(e)
	}
	q = qualityCommand()
	q.Version = 2
	if _, e = s.Execute(t.Context(), "actor", qualityProject, qualityPkg, q); e != domain.QualityForbidden {
		t.Fatal("verify granted edit", e)
	}
	f.caps.Scoped = false
	w, e = s.Workspace(t.Context(), "actor", qualityProject, qualityPkg)
	if e != nil || !w.TaskOnly || len(w.History) != 0 || w.Record == nil {
		t.Fatal("task history leaked", e)
	}
	f.caps.Write = true
	f.source.ReportVersion++
	w, e = s.Workspace(t.Context(), "actor", qualityProject, qualityPkg)
	if e != nil || len(w.CertificateActions["TC"]) != 0 || len(w.Actions) != 1 || w.Eligibility.Ready {
		t.Fatal("obsolete source permitted", e)
	}
	w, e = s.Execute(t.Context(), "actor", qualityProject, qualityPkg, domain.QualityCommand{Version: 2, Action: "revise", Reason: "Review corrected IRN"})
	if e != nil || w.Record.Revision != 2 || w.Record.Certificates[0].Verified || w.Record.Certificates[0].Number != "TC-1" {
		t.Fatal("revision lost metadata or retained attestation", e)
	}
}
