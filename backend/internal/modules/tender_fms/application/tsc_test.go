package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
	"testing"
)

type tscWorkerRepository struct {
	ports.TSCRepository
	job    domain.TSCJob
	result domain.TSCResult
	code   string
}

func (r *tscWorkerRepository) Claim(context.Context) (*domain.TSCJob, error)   { return &r.job, nil }
func (r *tscWorkerRepository) Authorize(context.Context, string, string) error { return nil }
func (r *tscWorkerRepository) Renew(context.Context, domain.TSCJob) error      { return nil }
func (r *tscWorkerRepository) Finish(_ context.Context, _ domain.TSCJob, result domain.TSCResult, code string) error {
	r.result = result
	r.code = code
	return nil
}

type tscWorkerDrafts struct{ ports.DraftRepository }

func (tscWorkerDrafts) Authorize(context.Context, string, bool) error { return nil }
func (tscWorkerDrafts) Get(context.Context, string, string) (domain.DocumentDraft, error) {
	return domain.DocumentDraft{Version: 1}, nil
}

type tscWorkerUnit struct{ repository *tscWorkerRepository }

func (u tscWorkerUnit) Within(ctx context.Context, f func(context.Context, ports.TSCRepository, ports.DraftRepository) error) error {
	return f(ctx, u.repository, tscWorkerDrafts{})
}

type tscWorkerProvider struct{ failure error }

func (tscWorkerProvider) Model() string { return "fixture" }
func (p tscWorkerProvider) Propose(_ context.Context, q domain.TSCQuestion, _ []domain.TSCPassage) (domain.TSCAnswer, error) {
	if q.ID == "particulars.q01" {
		return domain.TSCAnswer{}, p.failure
	}
	return domain.TSCAnswer{QuestionID: q.ID, Status: "answered", Text: "Synthetic evidence", Citations: []domain.TSCCitation{{SourceID: "source", Quote: "Synthetic evidence"}}}, nil
}
func TestTSCWorkerRetainsValidAnswersAfterRejectedProposal(t *testing.T) {
	r := &tscWorkerRepository{job: domain.TSCJob{Mode: "ollama", Model: "fixture", DraftVersion: 1, Input: domain.TSCInput{Questions: []domain.TSCQuestion{{ID: "particulars.q01"}, {ID: "particulars.q02"}}, Passages: []domain.TSCPassage{{ID: "source", Text: "Synthetic evidence"}}}}}
	s := NewTSC(tscWorkerUnit{r}, nil, tscWorkerProvider{domain.TSCProviderInvalidProposal}, nil)
	worked, err := s.Once(context.Background())
	if err != nil || !worked || r.code != "" || len(r.result.Answers) != 2 {
		t.Fatalf("worker failed: %v %s", err, r.code)
	}
	if r.result.Answers[0].Status != "needs_review" || r.result.Answers[0].Origin != "rejected_ollama_proposal" || len(r.result.Answers[0].Citations) != 0 || r.result.Answers[1].Status != "answered" || len(r.result.Warnings) != 1 {
		t.Fatal("rejected evidence was retained or valid answer lost")
	}
}
func TestTSCWorkerClassifiesProviderFailures(t *testing.T) {
	for _, failure := range []domain.TSCProviderError{domain.TSCProviderTimeout, domain.TSCProviderUnavailable, domain.TSCProviderModelUnavailable} {
		t.Run(string(failure), func(t *testing.T) {
			r := &tscWorkerRepository{job: domain.TSCJob{Mode: "ollama", Model: "fixture", DraftVersion: 1, Input: domain.TSCInput{Questions: []domain.TSCQuestion{{ID: "particulars.q01"}}}}}
			_, err := NewTSC(tscWorkerUnit{r}, nil, tscWorkerProvider{failure}, nil).Once(context.Background())
			if err != nil || r.code != string(failure) {
				t.Fatalf("lost safe failure category: %v %s", err, r.code)
			}
		})
	}
}

func TestTSCWorkerUsesDocumentWideLabelledCandidate(t *testing.T) {
	r := &tscWorkerRepository{job: domain.TSCJob{Mode: "ollama", Model: "fixture", DraftVersion: 1, Input: domain.TSCInput{Questions: []domain.TSCQuestion{{ID: "particulars.q01"}}, Passages: []domain.TSCPassage{{ID: "source", Text: "Employer: Synthetic Electrical Works Department"}}}}}
	_, err := NewTSC(tscWorkerUnit{r}, nil, tscWorkerProvider{domain.TSCProviderUnavailable}, nil).Once(context.Background())
	if err != nil || r.code != "" || len(r.result.Answers) != 1 || r.result.Answers[0].Origin != "labelled_source_candidate" || r.result.Answers[0].Status != "answered" {
		t.Fatal("labelled source candidate was lost", err, r.code)
	}
}
