package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
	"testing"
)

const originalDraftID = "11111111-1111-4111-8111-111111111111"
const originalJobID = "22222222-2222-4222-8222-222222222222"
const originalKey = "33333333-3333-4333-8333-333333333333"

type originalRepo struct {
	ports.TSCRepository
	j         domain.TSCJob
	allowed   bool
	call      domain.TSCCloudCall
	ready     bool
	completed int
}

func (r *originalRepo) Authorize(_ context.Context, _ string, action string) error {
	if action == "originals.write" && !r.allowed {
		return domain.Forbidden
	}
	return nil
}
func (r *originalRepo) Get(context.Context, string, string) (domain.TSCJob, error) { return r.j, nil }
func (r *originalRepo) ReserveCloud(_ context.Context, c domain.TSCCloudCall) (domain.TSCCloudCall, bool, error) {
	if r.ready {
		return r.call, false, nil
	}
	r.call = c
	return c, true, nil
}
func (r *originalRepo) CompleteCloud(_ context.Context, c domain.TSCCloudCall, a domain.TSCAnswer, code string) error {
	r.completed++
	c.Answer = a
	c.State = "ready"
	r.call = c
	r.ready = true
	return nil
}

type originalDrafts struct {
	ports.DraftRepository
	reads int
}

func (*originalDrafts) Authorize(context.Context, string, bool) error { return nil }
func (*originalDrafts) Get(context.Context, string, string) (domain.DocumentDraft, error) {
	return domain.DocumentDraft{ID: originalDraftID, Version: 1}, nil
}
func (d *originalDrafts) Artifact(context.Context, domain.DocumentDraft, string, string) ([]byte, error) {
	d.reads++
	return []byte("%PDF-synthetic"), nil
}

type originalUnit struct {
	r *originalRepo
	d *originalDrafts
}

func (u originalUnit) Within(ctx context.Context, f func(context.Context, ports.TSCRepository, ports.DraftRepository) error) error {
	return f(ctx, u.r, u.d)
}

type originalIdentity struct{}

func (originalIdentity) Resolve(context.Context, string) (domain.Principal, error) {
	return domain.Principal{ID: "actor", Active: true}, nil
}

type originalProvider struct {
	calls int
	after func()
}

func (*originalProvider) Model() string         { return "fixture" }
func (*originalProvider) OriginalModel() string { return "fixture-originals" }
func (*originalProvider) Propose(context.Context, domain.TSCQuestion, []domain.TSCPassage) (domain.TSCAnswer, error) {
	panic("excerpt fallback must not run")
}
func (p *originalProvider) ProposeOriginals(_ context.Context, q domain.TSCQuestion, docs []domain.TSCOriginal, _ []domain.TSCPassage) (domain.TSCAnswer, error) {
	p.calls++
	if len(docs) != 1 {
		panic("source omitted")
	}
	if p.after != nil {
		p.after()
	}
	return domain.TSCAnswer{QuestionID: q.ID, Status: "answered", Text: "INR 2500", Citations: []domain.TSCCitation{{SourceID: "s", Quote: "Earnest money INR 2500"}}}, nil
}
func originalService() (*TSC, *originalRepo, *originalDrafts, *originalProvider) {
	r := &originalRepo{allowed: true, j: domain.TSCJob{ID: originalJobID, DraftID: originalDraftID, State: "ready", Version: 1, DraftVersion: 1, Input: domain.TSCInput{Questions: []domain.TSCQuestion{{ID: "emd.q05", Text: "EMD amount"}}, Passages: []domain.TSCPassage{{ID: "s", DocumentID: "doc", Text: "Earnest money INR 2500"}}, Sources: []domain.TSCSnapshotSource{{Source: domain.DraftSource{ID: "doc", Path: "source.pdf", SHA256: "0000000000000000000000000000000000000000000000000000000000000000", ExpectedPages: 1}}}}}}
	d := &originalDrafts{}
	p := &originalProvider{}
	s := NewTSC(originalUnit{r, d}, originalIdentity{}, nil, nil).WithProviders(map[string]ports.TSCProvider{"gemini": p})
	return s, r, d, p
}
func TestOriginalsRequireConsentAndDedicatedPermissionBeforeReading(t *testing.T) {
	s, r, d, p := originalService()
	if _, e := s.Research(context.Background(), "token", originalDraftID, originalJobID, "emd.q05", "", "gemini_originals", originalKey, 1); e == nil || d.reads != 0 || p.calls != 0 {
		t.Fatal("missing confirmation allowed upload")
	}
	r.allowed = false
	if _, e := s.Research(context.Background(), "token", originalDraftID, originalJobID, "emd.q05", "", "gemini_originals", originalKey, 1, domain.TSCResearchOptions{OriginalDocumentsConfirmed: true}); e != domain.Forbidden || d.reads != 0 || p.calls != 0 {
		t.Fatal("permission bypass", e)
	}
}
func TestOriginalsReplayDoesNotSendOrChargeAgain(t *testing.T) {
	s, r, _, p := originalService()
	for i := 0; i < 2; i++ {
		out, e := s.Research(context.Background(), "token", originalDraftID, originalJobID, "emd.q05", "", "gemini_originals", originalKey, 1, domain.TSCResearchOptions{OriginalDocumentsConfirmed: true})
		if e != nil || out.Proposal == nil || out.Proposal.Accepted {
			t.Fatal(e)
		}
	}
	if p.calls != 1 || r.completed != 1 || r.call.ReservedPaise != 500 || !r.call.Input.OriginalDocumentsConfirmed {
		t.Fatal("paid replay/audit lost")
	}
}
func TestOriginalsRevocationDuringInferenceBlocksReturnAndCache(t *testing.T) {
	s, r, _, p := originalService()
	p.after = func() { r.allowed = false }
	out, e := s.Research(context.Background(), "token", originalDraftID, originalJobID, "emd.q05", "", "gemini_originals", originalKey, 1, domain.TSCResearchOptions{OriginalDocumentsConfirmed: true})
	if e != domain.Forbidden || out.Proposal != nil || r.completed != 0 {
		t.Fatal("revoked original permission bypass", e)
	}
}
func TestOriginalsRejectUnsupportedPackageWithoutCloudCalls(t *testing.T) {
	s, r, _, p := originalService()
	r.j.Input.Sources[0].Source.Path = "not-pdf.docx"
	if _, e := s.Research(context.Background(), "token", originalDraftID, originalJobID, "emd.q05", "", "gemini_originals", originalKey, 1, domain.TSCResearchOptions{OriginalDocumentsConfirmed: true}); e == nil || p.calls != 0 || r.completed != 0 {
		t.Fatal("unsupported source silently omitted")
	}
}
