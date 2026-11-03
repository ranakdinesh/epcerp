package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
	"testing"
)

type deadlineRanker struct{ cancel context.CancelFunc }

func (*deadlineRanker) Model() string { return "blocking-test-model" }
func (r *deadlineRanker) Rank(ctx context.Context, _ domain.TSCQuestion, _ []domain.TSCPassage) ([]domain.TSCRank, error) {
	if r.cancel != nil {
		r.cancel()
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

type completionUnit struct{ repo *tscWorkerRepository }

func (u completionUnit) Within(ctx context.Context, f func(context.Context, ports.TSCRepository, ports.DraftRepository) error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return f(ctx, u.repo, tscWorkerDrafts{})
}
func guardJob() domain.TSCJob {
	return domain.TSCJob{Mode: "local", DraftVersion: 1, Input: domain.TSCInput{RetrievalModel: "blocking-test-model", Questions: []domain.TSCQuestion{{ID: "particulars.q01"}, {ID: "emd.q05", Text: "EMD amount", Section: "emd"}, {ID: "emd.q06", Text: "EMD payment", Section: "emd"}}, Passages: []domain.TSCPassage{{ID: "a", Text: "Employer: Synthetic Department"}, {ID: "b", Text: "Earnest money deposit INR 5000 is payable online."}}}}
}
func TestDeadlinePersistsPartialDraftUsingFreshCompletionContext(t *testing.T) {
	r := &tscWorkerRepository{job: guardJob()}
	s := NewTSC(completionUnit{r}, nil, nil, nil).WithEvidence(NewTSCEvidence(&deadlineRanker{}))
	s.budget.Work = 10_000_000 // 10ms test deadline
	worked, err := s.Once(context.Background())
	if !worked || err != nil || r.code != "" || len(r.result.Answers) != 3 {
		t.Fatalf("partial result not persisted: %v %s", err, r.code)
	}
	if r.result.Answers[0].Status != "answered" || r.result.Answers[0].Accepted {
		t.Fatal("completed fact lost or approved")
	}
	for _, a := range r.result.Answers[1:] {
		if a.Status != "needs_review" || a.Origin != "processing_time_limit" || a.Text != "" || a.Accepted {
			t.Fatal("unfinished answer fabricated", a)
		}
	}
	if len(r.result.Warnings) == 0 || r.result.Warnings[len(r.result.Warnings)-1] != "Processing time limit reached; unfinished questions require source review. No automatic paid retry." {
		t.Fatal("partial completion hidden")
	}
}
func TestShutdownPersistsTerminalFailureDespiteCanceledWorkContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &tscWorkerRepository{job: guardJob()}
	s := NewTSC(completionUnit{r}, nil, nil, nil).WithEvidence(NewTSCEvidence(&deadlineRanker{cancel}))
	if _, err := s.Once(ctx); err != nil || r.code != "processing_interrupted" {
		t.Fatal("cancellation left job leased", err, r.code)
	}
}
func TestProviderWarningCodeAllowlistNeverLeaksDetail(t *testing.T) {
	if got := domain.TSCProviderFailure(domain.CaptureFailure(502, "provider_rate_limited", "private payload")); got != "provider_rate_limited" {
		t.Fatal(got)
	}
	if got := domain.TSCProviderFailure(errors.New("private payload")); got != "provider_failed" {
		t.Fatal(got)
	}
}

type revokingRanker struct{ revoked *bool }

func (*revokingRanker) Model() string { return "blocking-test-model" }
func (r *revokingRanker) Rank(ctx context.Context, _ domain.TSCQuestion, _ []domain.TSCPassage) ([]domain.TSCRank, error) {
	<-ctx.Done()
	*r.revoked = true
	return nil, ctx.Err()
}

type revokedCompletionDrafts struct {
	tscWorkerDrafts
	revoked *bool
}

func (d revokedCompletionDrafts) Authorize(context.Context, string, bool) error {
	if *d.revoked {
		return domain.Forbidden
	}
	return nil
}

type revokedCompletionUnit struct {
	repo    *tscWorkerRepository
	revoked *bool
}

func (u revokedCompletionUnit) Within(ctx context.Context, f func(context.Context, ports.TSCRepository, ports.DraftRepository) error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return f(ctx, u.repo, revokedCompletionDrafts{revoked: u.revoked})
}
func TestPartialCompletionStillRejectsRevokedSourceAccess(t *testing.T) {
	revoked := false
	r := &tscWorkerRepository{job: guardJob()}
	s := NewTSC(revokedCompletionUnit{r, &revoked}, nil, nil, nil).WithEvidence(NewTSCEvidence(&revokingRanker{&revoked}))
	s.budget.Work = 10_000_000
	if _, err := s.Once(context.Background()); err != nil || r.code != "source_or_access_changed" {
		t.Fatal("detached completion bypassed authorization", err, r.code)
	}
}

type stoppedCloudRepo struct {
	tscWorkerRepository
	reserved int
}

func (r *stoppedCloudRepo) ReserveCloud(_ context.Context, c domain.TSCCloudCall) (domain.TSCCloudCall, bool, error) {
	r.reserved++
	return c, true, nil
}
func (*stoppedCloudRepo) CompleteCloud(context.Context, domain.TSCCloudCall, domain.TSCAnswer, string) error {
	return nil
}

type stoppedCloudUnit struct{ r *stoppedCloudRepo }

func (u stoppedCloudUnit) Within(ctx context.Context, f func(context.Context, ports.TSCRepository, ports.DraftRepository) error) error {
	return f(ctx, u.r, tscWorkerDrafts{})
}

type rateLimitedCloud struct{ calls int }

func (*rateLimitedCloud) Model() string { return "fixture" }
func (p *rateLimitedCloud) Propose(context.Context, domain.TSCQuestion, []domain.TSCPassage) (domain.TSCAnswer, error) {
	p.calls++
	return domain.TSCAnswer{}, domain.TSCProviderRateLimited
}
func TestRateLimitPausesFurtherCloudReservationsAndKeepsReviewableResults(t *testing.T) {
	r := &stoppedCloudRepo{}
	r.job = guardJob()
	r.job.Mode = "gemini"
	r.job.Model = "fixture"
	r.job.Input.RetrievalModel = ""
	r.job.Input.Questions = r.job.Input.Questions[1:]
	p := &rateLimitedCloud{}
	s := NewTSC(stoppedCloudUnit{r}, nil, nil, nil).WithProviders(map[string]ports.TSCProvider{"gemini": p})
	if _, err := s.Once(context.Background()); err != nil || r.code != "" || p.calls != 1 || r.reserved != 1 || len(r.result.Answers) != 2 {
		t.Fatal("cloud retry/extra reservation or lost result", err, p.calls, r.reserved)
	}
	for _, a := range r.result.Answers {
		if a.Status != "needs_review" || a.Accepted || a.Text != "" {
			t.Fatal("cloud failure fabricated a fact", a)
		}
	}
}
