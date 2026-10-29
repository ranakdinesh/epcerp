package integration

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	tenderout "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound"
	tenderapp "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type nativeCloudProposal struct {
	calls atomic.Int64
	after func()
}

func (p *nativeCloudProposal) Model() string { return "synthetic-cloud-model" }
func (p *nativeCloudProposal) Propose(_ context.Context, q domain.TSCQuestion, all []domain.TSCPassage) (domain.TSCAnswer, error) {
	p.calls.Add(1)
	if p.after != nil {
		p.after()
	}
	selected := domain.RetrieveTSCEvidence(q, all)
	if len(selected) == 0 {
		return domain.EmptyTSCAnswer(q), nil
	}
	s := selected[0]
	return domain.TSCAnswer{QuestionID: q.ID, Status: "answered", Text: "Synthetic source-backed proposal", Origin: "synthetic", Citations: []domain.TSCCitation{{SourceID: s.ID, Quote: s.Text}}}, nil
}

func verifyTSCCloudNative(t *testing.T, ctx context.Context, pool *pgxpool.Pool, unit *tenderout.TSCUnit, service *tenderapp.TSC, j domain.TSCJob, owner, role string) {
	t.Helper()
	provider := &nativeCloudProposal{}
	service.WithProviders(map[string]ports.TSCProvider{"gemini": provider})
	query := strings.Join(strings.Fields(j.Input.Passages[0].Text)[:min(4, len(strings.Fields(j.Input.Passages[0].Text)))], " ")
	key := tenderout.Values{}.ID()
	search, e := service.Research(ctx, "owner", j.DraftID, j.ID, "particulars.q01", query, "search", "", j.Version)
	if e != nil || len(search.Passages) == 0 || provider.calls.Load() != 0 {
		t.Fatal("free source search", e)
	}
	first, e := service.Research(ctx, "owner", j.DraftID, j.ID, "particulars.q01", query, "gemini", key, j.Version)
	if e != nil || first.Proposal == nil || provider.calls.Load() != 1 {
		t.Fatal("cloud proposal", e)
	}
	if _, e = service.Research(ctx, "owner", j.DraftID, j.ID, "particulars.q01", query, "gemini", key, j.Version); e != nil || provider.calls.Load() != 1 {
		t.Fatal("paid replay resent", e)
	}
	if _, e = service.Research(ctx, "outsider", j.DraftID, j.ID, "particulars.q01", query, "gemini", key, j.Version); !errors.Is(e, domain.Missing) {
		t.Fatal("cross-owner proposal leaked", e)
	}
	var encrypted []byte
	pool.QueryRow(ctx, "SELECT result_cipher FROM tender.tsc_cloud_calls WHERE draft_id=$1", j.DraftID).Scan(&encrypted)
	if len(encrypted) == 0 || strings.Contains(string(encrypted), "Synthetic source-backed") {
		t.Fatal("cloud proposal not encrypted")
	}
	// A permission revoked during inference prevents returning or caching an answer.
	provider.after = func() {
		execFixture(t, pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='tenders.tsc.write')", role)
	}
	if _, e = service.Research(ctx, "owner", j.DraftID, j.ID, "particulars.q01", query, "gemini", tenderout.Values{}.ID(), j.Version); !errors.Is(e, domain.Forbidden) {
		t.Fatal("revoked cloud answer escaped", e)
	}
	provider.after = nil
	execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='tenders.tsc.write'", role)
	rollback := errors.New("reservation rollback")
	e = unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, _ ports.DraftRepository) error {
		_, _, e := r.ReserveCloud(ctx, domain.TSCCloudCall{DraftID: j.DraftID, JobID: j.ID, ActorID: owner, Key: tenderout.Values{}.ID(), Provider: "gemini", Model: provider.Model(), ReservedPaise: 150})
		if e != nil {
			return e
		}
		return rollback
	})
	if !errors.Is(e, rollback) {
		t.Fatal("reservation rollback", e)
	}
	before, e := service.Options(ctx, "owner", j.DraftID)
	if e != nil || before.ReservedPaise != 300 {
		t.Fatal("rolled-back reservation charged", before, e)
	}
	// Independent transactions compete for the remaining cumulative allowance.
	var successes atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 75; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, _ ports.DraftRepository) error {
				_, fresh, e := r.ReserveCloud(ctx, domain.TSCCloudCall{DraftID: j.DraftID, JobID: j.ID, ActorID: owner, Key: tenderout.Values{}.ID(), Fingerprint: "fixture", Provider: "gemini", Model: provider.Model(), ReservedPaise: 150})
				if e == nil && fresh {
					successes.Add(1)
				}
				return e
			})
			if e != nil {
				var de *domain.CaptureError
				if !errors.As(e, &de) {
					t.Error("reservation concurrency", e)
				}
			}
		}()
	}
	wg.Wait()
	options, e := service.Options(ctx, "owner", j.DraftID)
	if e != nil || options.ReservedPaise != 9900 || successes.Load() != 64 {
		t.Fatal("budget ceiling/concurrency", options, successes.Load(), e)
	}
	if _, e = service.Research(ctx, "owner", j.DraftID, j.ID, "particulars.q01", query, "gemini", tenderout.Values{}.ID(), j.Version); e == nil {
		t.Fatal("overspend accepted")
	}
	cloudJob, e := service.Create(ctx, "owner", j.DraftID, tenderout.Values{}.ID(), "gemini", j.DraftVersion)
	if e != nil {
		t.Fatal("cloud generation create", e)
	}
	if _, e = service.Once(ctx); e != nil {
		t.Fatal("bounded cloud worker", e)
	}
	partial, e := service.Get(ctx, "owner", j.DraftID, cloudJob.ID)
	if e != nil || partial.State != "ready" || len(partial.Result.Answers) != 70 || len(partial.Result.Warnings) == 0 || provider.calls.Load() != 2 {
		t.Fatal("allowance exhaustion discarded draft or resent requests", e, partial.State, provider.calls.Load())
	}
	t.Log("Cloud native: free search, exact scope, encrypted replay without resend, live revocation and concurrent INR 100 ceiling passed")
}
