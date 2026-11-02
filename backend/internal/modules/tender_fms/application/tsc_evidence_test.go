package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
	"testing"
)

type evidenceRanker struct {
	failure bool
	calls   int
}

func (*evidenceRanker) Model() string { return "fixture-ranker-v1" }
func (r *evidenceRanker) Rank(_ context.Context, _ domain.TSCQuestion, p []domain.TSCPassage) ([]domain.TSCRank, error) {
	r.calls++
	if r.failure {
		return nil, errors.New("private error must not escape")
	}
	out := []domain.TSCRank{}
	for _, v := range p {
		out = append(out, domain.TSCRank{ID: v.ID, Score: 1})
	}
	return out, nil
}
func TestEvidenceFallbackIsVisibleAndHumanDecisionNeverInferred(t *testing.T) {
	r := &evidenceRanker{failure: true}
	s := NewTSCEvidence(r)
	p := []domain.TSCPassage{{ID: "a", Text: "Earnest money INR 5000"}}
	out := s.Select(context.Background(), domain.TSCQuestion{ID: "emd.q05", Text: "EMD", Section: "emd"}, p)
	if out.Diagnostic.Fallback != "local_ranking_unavailable" || len(out.Passages) != 1 {
		t.Fatal("fallback not preserved")
	}
	s.Select(context.Background(), domain.TSCQuestion{ID: "conclusion.q02"}, p)
	if r.calls != 1 {
		t.Fatal("human judgement sent to model")
	}
}
func TestLocalModeExtractsFactsKeepsPendingEvidenceAndNeverCallsProvider(t *testing.T) {
	ranker := &evidenceRanker{}
	r := &tscWorkerRepository{job: domain.TSCJob{Mode: "local", DraftVersion: 1, Input: domain.TSCInput{RetrievalModel: ranker.Model(), Questions: []domain.TSCQuestion{{ID: "particulars.q01"}, {ID: "emd.q05", Section: "emd", Text: "EMD amount"}, {ID: "conclusion.q02"}}, Passages: []domain.TSCPassage{{ID: "a", Text: "Employer: Synthetic Electrical Department"}, {ID: "b", Text: "Earnest money deposit INR 5000 payable online."}}}}}
	s := NewTSC(tscWorkerUnit{r}, nil, nil, nil).WithEvidence(NewTSCEvidence(ranker))
	_, err := s.Once(context.Background())
	if err != nil || r.code != "" || len(r.result.Answers) != 3 || len(r.result.Evidence) != 3 {
		t.Fatal("local draft failed", err, r.code)
	}
	if r.result.Answers[0].Status != "answered" || r.result.Answers[0].Accepted || r.result.Answers[1].Status != "needs_review" || len(r.result.Evidence[1].Passages) == 0 || r.result.Answers[2].Status != "needs_review" {
		t.Fatal("unsupported fact, missing pending evidence or approval")
	}
	if ranker.calls != 1 {
		t.Fatal("known fact or human decision redundantly ranked", ranker.calls)
	}
}

type selectionCloudRepo struct {
	ports.TSCRepository
	call domain.TSCCloudCall
	done bool
}

func (*selectionCloudRepo) Authorize(context.Context, string, string) error { return nil }
func (r *selectionCloudRepo) ReserveCloud(_ context.Context, c domain.TSCCloudCall) (domain.TSCCloudCall, bool, error) {
	r.call = c
	return c, true, nil
}
func (r *selectionCloudRepo) CompleteCloud(_ context.Context, _ domain.TSCCloudCall, _ domain.TSCAnswer, _ string) error {
	r.done = true
	return nil
}

type selectionCloudUnit struct{ r *selectionCloudRepo }

func (u selectionCloudUnit) Within(ctx context.Context, f func(context.Context, ports.TSCRepository, ports.DraftRepository) error) error {
	return f(ctx, u.r, tscWorkerDrafts{})
}

type selectionCloudProvider struct{ sent []domain.TSCPassage }

func (*selectionCloudProvider) Model() string { return "fixture" }
func (p *selectionCloudProvider) Propose(_ context.Context, q domain.TSCQuestion, passages []domain.TSCPassage) (domain.TSCAnswer, error) {
	p.sent = passages
	return domain.TSCAnswer{QuestionID: q.ID, Status: "answered", Text: "INR 5000", Citations: []domain.TSCCitation{{SourceID: passages[0].ID, Quote: passages[0].Text}}}, nil
}
func TestCloudReservationAndProviderUseIdenticalSelectedEvidence(t *testing.T) {
	r := &selectionCloudRepo{}
	p := &selectionCloudProvider{}
	s := NewTSC(selectionCloudUnit{r}, nil, nil, nil).WithProviders(map[string]ports.TSCProvider{"gemini": p})
	q := domain.TSCQuestion{ID: "emd.q05", Section: "emd", Text: "EMD amount"}
	chosen := domain.TSCPassage{ID: "chosen", Text: "EMD INR 5000"}
	j := domain.TSCJob{DraftVersion: 1, Input: domain.TSCInput{Questions: []domain.TSCQuestion{q}, Passages: []domain.TSCPassage{chosen, {ID: "irrelevant", Text: "Unrelated retained source"}}}}
	selected := domain.TSCSelection{Passages: []domain.TSCPassage{chosen}, Diagnostic: domain.TSCQuestionDiagnostic{Method: "bm25_minilm", Model: "pinned-fixture"}}
	if _, e := s.cloudPropose(context.Background(), j, q, "gemini", "fixture-key", 0, selected); e != nil {
		t.Fatal(e)
	}
	if !r.done || len(p.sent) != 1 || p.sent[0] != chosen || len(r.call.Input.Passages) != 1 || r.call.Input.Passages[0] != chosen || r.call.Input.Retrieval.Model != "pinned-fixture" {
		t.Fatal("provider evidence differs from reservation or irrelevant source sent")
	}
}

func TestQueuedSnapshotCannotSilentlyUseAnotherExtractionVersion(t *testing.T) {
	ranker := &evidenceRanker{}
	r := &tscWorkerRepository{job: domain.TSCJob{Mode: "local", DraftVersion: 1, Input: domain.TSCInput{ProposalVersion: "tsc-local-rag-v4", Questions: []domain.TSCQuestion{{ID: "particulars.q01"}}, Passages: []domain.TSCPassage{{ID: "a", Text: "Employer: Synthetic Board"}}}}}
	_, err := NewTSC(tscWorkerUnit{r}, nil, nil, nil).WithEvidence(NewTSCEvidence(ranker)).Once(context.Background())
	if err != nil || r.code != "configured_extraction_changed" || len(r.result.Answers) != 0 || ranker.calls != 0 {
		t.Fatal("snapshot version changed silently", err, r.code)
	}
}
