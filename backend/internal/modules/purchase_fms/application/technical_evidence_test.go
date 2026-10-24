package application

import (
	"context"
	"errors"
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type technicalTestCipher struct{}

func (technicalTestCipher) Seal(v []byte) ([]byte, string, error) {
	return append([]byte("encrypted:"), v...), "test-digest", nil
}
func (technicalTestCipher) Open(v []byte, hash string) ([]byte, error) {
	if hash != "test-digest" || len(v) < 10 || string(v[:10]) != "encrypted:" {
		return nil, errors.New("corrupt evidence")
	}
	return append([]byte{}, v[10:]...), nil
}

type technicalTestInspector struct{ calls int }

func (i *technicalTestInspector) Inspect(_, _ string, _ []byte) error { i.calls++; return nil }
func TestTechnicalEvidenceOwnershipVersionAuthorizationAndAtomicRead(t *testing.T) {
	f := technicalNewFixture()
	service := NewTechnical(f, f, f)
	w, e := service.Execute(t.Context(), "actor", technicalId(3), technicalId(4), domain.TechnicalCommand{Action: "configure", Reason: "Actual kit"})
	if e != nil {
		t.Fatal(e)
	}
	inspector := &technicalTestInspector{}
	files := NewTechnicalEvidence(service, technicalTestCipher{}, inspector)
	q := domain.TechnicalUpload{Version: w.Case.Version, Purpose: "document", Name: "actual.pdf", MediaType: "application/pdf", Reason: " Actual vendor evidence ", Data: []byte("actual evidence bytes")}
	out, e := files.Upload(t.Context(), "actor", technicalId(3), technicalId(4), q)
	if e != nil || out.ID == "" || len(f.fileEvents) != 1 || f.fileEvents[0].Reason != "Actual vendor evidence" || f.record.Documents[0].Status != "pending" {
		t.Fatal(out, e)
	}
	evidence := f.evidence[out.ID]
	if string(evidence.Ciphertext) == string(q.Data) {
		t.Fatal("plaintext stored")
	}
	downloaded, data, e := files.Download(t.Context(), "actor", technicalId(3), technicalId(4), out.ID)
	if e != nil || downloaded.ID != out.ID || string(data) != string(q.Data) || len(f.fileEvents) != 2 {
		t.Fatal(downloaded, e)
	}
	f.fail = true
	if v, e := files.Upload(t.Context(), "actor", technicalId(3), technicalId(4), q); e == nil || v.ID != "" || len(f.evidence) != 1 || len(f.fileEvents) != 2 {
		t.Fatal("failed file commit published evidence", e)
	}
	if v, data, e := files.Download(t.Context(), "actor", technicalId(3), technicalId(4), out.ID); e == nil || v.ID != "" || data != nil || len(f.fileEvents) != 2 {
		t.Fatal("failed read audit exposed bytes", e)
	}
	f.fail = false
	q.Version++
	if _, e := files.Upload(t.Context(), "actor", technicalId(3), technicalId(4), q); e == nil {
		t.Fatal("stale kit upload accepted")
	}
	q.Version--
	f.source.Available = false
	if _, e := files.Upload(t.Context(), "actor", technicalId(3), technicalId(4), q); e == nil {
		t.Fatal("held source upload accepted")
	}
	f.source.Available = true
	f.caps.Work = map[int]bool{7: true}
	calls := inspector.calls
	if _, e := files.Upload(t.Context(), "actor", technicalId(3), technicalId(4), q); e == nil || inspector.calls != calls {
		t.Fatal("unassigned uploader inspected content", e)
	}
	f.caps.Read = false
	if _, data, e := files.Download(t.Context(), "actor", technicalId(3), technicalId(4), out.ID); e == nil || data != nil {
		t.Fatal("revoked reader exposed bytes")
	}
	f.caps.Read = true
	if _, data, e := files.Download(t.Context(), "actor", technicalId(3), technicalId(9), out.ID); e == nil || data != nil {
		t.Fatal("foreign package evidence exposed")
	}
	evidence.File.CaseID = technicalId(9)
	f.evidence[out.ID] = evidence
	if _, data, e := files.Download(t.Context(), "actor", technicalId(3), technicalId(4), out.ID); e == nil || data != nil {
		t.Fatal("foreign file evidence exposed")
	}
}

type technicalObjectFixture struct {
	data []byte
	gets int
}

func (o *technicalObjectFixture) Put(_ context.Context, _, _ string, data []byte) error {
	o.data = append([]byte{}, data...)
	return nil
}
func (o *technicalObjectFixture) Get(_ context.Context, _, _ string) ([]byte, error) {
	o.gets++
	return append([]byte{}, o.data...), nil
}
func TestTechnicalNewUploadsUsePrivateObjectsAndRetainOnlyMetadata(t *testing.T) {
	f := technicalNewFixture()
	service := NewTechnical(f, f, f)
	w, e := service.Execute(t.Context(), "actor", technicalId(3), technicalId(4), domain.TechnicalCommand{Action: "configure", Reason: "Actual kit"})
	if e != nil {
		t.Fatal(e)
	}
	objects := &technicalObjectFixture{}
	files := NewTechnicalEvidence(service, technicalTestCipher{}, &technicalTestInspector{}).WithObjects(objects, "private-technical")
	q := domain.TechnicalUpload{Version: w.Case.Version, Purpose: "document", Name: "vendor.pdf", MediaType: "application/pdf", Reason: "Actual received drawing", Data: []byte("actual document bytes")}
	result, e := files.Upload(t.Context(), "actor", technicalId(3), technicalId(4), q)
	if e != nil {
		t.Fatal(e)
	}
	saved := f.evidence[result.ID]
	if len(saved.Ciphertext) != 0 || saved.ObjectKey == "" || saved.Bucket != "private-technical" || string(objects.data) == string(q.Data) {
		t.Fatal("private object location or encryption missing")
	}
	_, data, e := files.Download(t.Context(), "actor", technicalId(3), technicalId(4), result.ID)
	if e != nil || string(data) != string(q.Data) || objects.gets != 1 {
		t.Fatal("private object roundtrip", e)
	}
	f.caps.Read = false
	if _, _, e = files.Download(t.Context(), "actor", technicalId(3), technicalId(4), result.ID); e == nil || objects.gets != 1 {
		t.Fatal("revoked reader reached object storage")
	}
}
