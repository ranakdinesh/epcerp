package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func technicalId(i int) string {
	const digits = "0123456789abcdef"
	value := []byte("00000000-1111-4111-8111-111111111111")
	for index := 7; index >= 0; index-- {
		value[index] = digits[i&15]
		i >>= 4
	}
	return string(value)
}

type technicalFixture struct {
	record     *domain.TechnicalCase
	history    []domain.TechnicalEvent
	caps       domain.TechnicalAccess
	source     domain.TechnicalSource
	fail       bool
	counter    int
	evidence   map[string]domain.TechnicalEvidence
	fileEvents []domain.TechnicalFileEvent
}

func (f *technicalFixture) Within(ctx context.Context, work func(context.Context, ports.TechnicalRepository, ports.TechnicalContext) error) error {
	var before *domain.TechnicalCase
	if f.record != nil {
		c := domain.TechnicalCopy(*f.record)
		before = &c
	}
	history := append([]domain.TechnicalEvent{}, f.history...)
	evidence := map[string]domain.TechnicalEvidence{}
	for k, v := range f.evidence {
		evidence[k] = v
	}
	fileEvents := append([]domain.TechnicalFileEvent{}, f.fileEvents...)
	e := work(ctx, f, f)
	if e == nil && f.fail {
		e = errors.New("commit refused")
	}
	if e != nil {
		f.record = before
		f.history = history
		f.evidence = evidence
		f.fileEvents = fileEvents
	}
	return e
}
func (f *technicalFixture) Get(context.Context, string, string) (*domain.TechnicalCase, error) {
	if f.record == nil {
		return nil, nil
	}
	c := domain.TechnicalCopy(*f.record)
	return &c, nil
}
func (f *technicalFixture) Save(_ context.Context, c domain.TechnicalCase) (domain.TechnicalCase, error) {
	v := domain.TechnicalCopy(c)
	f.record = &v
	return domain.TechnicalCopy(v), nil
}
func (f *technicalFixture) Files(_ context.Context, caseID string) ([]domain.TechnicalFile, error) {
	files := []domain.TechnicalFile{}
	for _, v := range f.evidence {
		if v.File.CaseID == caseID {
			files = append(files, v.File)
		}
	}
	return files, nil
}
func (f *technicalFixture) Evidence(_ context.Context, caseID, fileID string) (*domain.TechnicalEvidence, error) {
	v, ok := f.evidence[fileID]
	if !ok || v.File.CaseID != caseID {
		return nil, nil
	}
	v.Ciphertext = append([]byte{}, v.Ciphertext...)
	return &v, nil
}
func (f *technicalFixture) StoreEvidence(_ context.Context, v domain.TechnicalEvidence) error {
	if f.evidence == nil {
		f.evidence = map[string]domain.TechnicalEvidence{}
	}
	v.Ciphertext = append([]byte{}, v.Ciphertext...)
	f.evidence[v.File.ID] = v
	return nil
}
func (f *technicalFixture) EvidenceEvent(_ context.Context, v domain.TechnicalFileEvent) error {
	f.fileEvents = append(f.fileEvents, v)
	return nil
}
func (f *technicalFixture) History(context.Context, string) ([]domain.TechnicalEvent, error) {
	return append([]domain.TechnicalEvent{}, f.history...), nil
}
func (f *technicalFixture) Event(_ context.Context, e domain.TechnicalEvent) error {
	f.history = append(f.history, e)
	return nil
}
func (f *technicalFixture) Access(context.Context, domain.Principal, string, string) (domain.TechnicalAccess, error) {
	return f.caps, nil
}
func (f *technicalFixture) Source(context.Context, string, string) (domain.TechnicalSource, error) {
	return f.source, nil
}
func (f *technicalFixture) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: technicalId(1), Active: true}, nil
}
func (f *technicalFixture) ID() string { f.counter++; return technicalId(f.counter + 100) }
func (f *technicalFixture) Now() domain.TechnicalTimestamp {
	return domain.TechnicalTimestamp{}.AddDate(2025, 9, 10).Add(10 * 60 * 60 * 1000000000)
}
func technicalNewFixture() *technicalFixture {
	return &technicalFixture{caps: domain.TechnicalAccess{Read: true, Write: true, Scoped: true, Work: map[int]bool{6: true}}, source: domain.TechnicalSource{CaseID: technicalId(6), Version: 11, ProjectID: technicalId(3), PackageID: technicalId(4), SupplierID: technicalId(5), LOIReference: "LOI-actual", Quantity: "10", Currency: "INR", Total: "100.00", FinancialReference: "actual-approval:fingerprint", ReleasedDate: "2026-10-11", Available: true}}
}
func TestTechnicalAtomicTechnicalCaseCreationAndFailedCommitExposeNoState(t *testing.T) {
	f := technicalNewFixture()
	service := NewTechnical(f, f, f)
	w, e := service.Execute(t.Context(), "actor", technicalId(3), technicalId(4), domain.TechnicalCommand{Action: "configure", Reason: " Actual source checklist "})
	if e != nil || w.Case == nil || w.Case.Version != 1 || len(w.History) != 1 || w.History[0].Reason != "Actual source checklist" {
		t.Fatal(w, e)
	}
	f.fail = true
	d := w.Case.Documents[0]
	required, active := true, true
	q := domain.TechnicalCommand{Version: 1, Action: "document", Reason: "Correct standard", Document: &domain.TechnicalDocumentInput{ID: d.ID, Kind: d.Kind, Title: d.Title, Category: d.Category, Standard: "Reviewed applicable specification", Required: &required, Active: &active, Status: "pending", Revision: "Rev 0"}}
	w, e = service.Execute(t.Context(), "actor", technicalId(3), technicalId(4), q)
	if e == nil || w.Case != nil || f.record.Version != 1 || len(f.history) != 1 || f.record.Documents[0].Standard != d.Standard {
		t.Fatal("failed commit leaked technical revision", e)
	}
}
func TestTechnicalNarrowTaskReadAndCurrentProjectSourceAreSeparateGuards(t *testing.T) {
	f := technicalNewFixture()
	service := NewTechnical(f, f, f)
	if _, e := service.Execute(t.Context(), "actor", technicalId(3), technicalId(4), domain.TechnicalCommand{Action: "configure", Reason: "Create"}); e != nil {
		t.Fatal(e)
	}
	f.caps = domain.TechnicalAccess{Read: true}
	w, e := service.Workspace(t.Context(), "actor", technicalId(3), technicalId(4))
	if e != nil || !w.TaskOnly || len(w.History) != 0 || len(w.Actions) != 0 {
		t.Fatal("task projection leaked history or authority", e)
	}
	f.caps = domain.TechnicalAccess{Read: true, Write: true, Scoped: true, Work: map[int]bool{6: true}}
	f.source.Available = false
	w, e = service.Workspace(t.Context(), "actor", technicalId(3), technicalId(4))
	if e != nil || w.Case == nil || len(w.Actions) != 0 || len(w.History) != 1 {
		t.Fatal("held source hid history or permitted execution", e)
	}
	f.source.PackageID = technicalId(9)
	w, e = service.Workspace(t.Context(), "actor", technicalId(3), technicalId(4))
	if e == nil || w.Case != nil {
		t.Fatal("foreign source published")
	}
}
