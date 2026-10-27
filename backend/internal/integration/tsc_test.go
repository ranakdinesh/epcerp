package integration

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	tenderhttp "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/inbound"
	tenderout "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound"
	tenderapp "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
	"net/http/httptest"
	"strings"
	"testing"
)

// Called with the real native package/reader fixture after successful reading.
func verifyTSCNative(t *testing.T, ctx context.Context, pool *pgxpool.Pool, drafts *tenderout.DraftUnit, identity projectIdentities, journal func(database.Connection) events.Writer, cipher *infrastructure.DocumentCipher, draft domain.DocumentDraft, owner, role string) {
	t.Helper()
	for _, code := range []string{"tenders.tsc.read", "tenders.tsc.write", "tenders.tsc.export"} {
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'Fixture') ON CONFLICT(code)DO NOTHING", tenderout.Values{}.ID(), code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	unit := tenderout.NewTSCUnit(drafts, cipher, journal)
	provider, e := tenderout.NewTSCOllama("", "")
	if e != nil {
		t.Fatal(e)
	}
	service := tenderapp.NewTSC(unit, tenderout.NewAuthBridge(identity), provider, tenderout.TSCExporter{})
	key := tenderout.Values{}.ID()
	job, e := service.Create(ctx, "owner", draft.ID, key, "manual", draft.Version)
	if e != nil {
		t.Fatal("TSC create", e)
	}
	again, e := service.Create(ctx, "owner", draft.ID, key, "manual", draft.Version)
	if e != nil || again.ID != job.ID {
		t.Fatal("TSC replay", e)
	}
	if _, e = service.Create(ctx, "owner", draft.ID, tenderout.Values{}.ID(), "manual", draft.Version); e == nil {
		t.Fatal("parallel pending generation allowed")
	}
	if _, e = service.Get(ctx, "outsider", draft.ID, job.ID); !errors.Is(e, domain.Missing) {
		t.Fatal("TSC leaked to outsider", e)
	}
	if worked, e := service.Once(ctx); !worked || e != nil {
		t.Fatal("TSC worker", e)
	}
	job, e = service.Get(ctx, "owner", draft.ID, job.ID)
	if e != nil || job.State != "ready" || len(job.Result.Answers) != 70 {
		t.Fatal("TSC result", e, job.State)
	}
	if job.Input.Coverage.Version != "document-evidence-v1" || job.Input.Coverage.Documents < 1 || job.Input.Coverage.Blocks < 1 || len(job.Result.Diagnostics) != 70 {
		t.Fatal("structured evidence/diagnostics missing")
	}
	markdown, e := service.ExportEvidence(ctx, "owner", draft.ID, job.ID)
	if e != nil || !strings.Contains(string(markdown), "TSC source evidence") || !strings.Contains(string(markdown), job.Input.Passages[0].ID) {
		t.Fatal("evidence Markdown export", e)
	}
	if _, e = service.ExportEvidence(ctx, "outsider", draft.ID, job.ID); !errors.Is(e, domain.Missing) {
		t.Fatal("evidence leaked outside draft scope", e)
	}
	router := chi.NewRouter()
	tenderhttp.RegisterTSC(router, service)
	for _, tc := range []struct {
		method, path, body, actor string
		status                    int
	}{
		{"GET", "/api/v1/tender-fms/drafts/" + draft.ID + "/tsc/" + job.ID, "", "owner", 200},
		{"GET", "/api/v1/tender-fms/drafts/" + draft.ID + "/tsc/" + job.ID, "", "outsider", 404},
		{"GET", "/api/v1/tender-fms/drafts/" + draft.ID + "/tsc/" + job.ID + "/evidence", "", "owner", 200},
		{"GET", "/api/v1/tender-fms/drafts/" + draft.ID + "/tsc/" + job.ID + "/evidence", "", "outsider", 404},
		{"POST", "/api/v1/tender-fms/drafts/" + draft.ID + "/tsc", `{"mode":"manual","expected_version":1,"approve":true}`, "owner", 422},
		{"PATCH", "/api/v1/tender-fms/drafts/" + draft.ID + "/tsc/" + job.ID + "/answers/particulars.q01", `{"approved":true}`, "owner", 422},
		{"GET", "/api/v1/tender-fms/drafts/" + draft.ID + "/tsc/options", "", "owner", 200},
		{"GET", "/api/v1/tender-fms/drafts/" + draft.ID + "/tsc/options", "", "outsider", 404},
		{"POST", "/api/v1/tender-fms/drafts/" + draft.ID + "/tsc/" + job.ID + "/answers/particulars.q01/research", `{"mode":"search","approved":true}`, "owner", 422},
	} {
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		request.Header.Set("Authorization", "Bearer "+tc.actor)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("TSC HTTP %s expected %d got %d", tc.path, tc.status, response.Code)
		}
	}
	passage := job.Input.Passages[0]
	verifyTSCCloudNative(t, ctx, pool, unit, service, job, owner, role)
	quote := passage.Text
	if len(quote) > 2000 {
		t.Fatal("fixture excerpt exceeds bound")
	}
	answer := domain.TSCAnswer{QuestionID: "particulars.q01", Status: "answered", Text: "Synthetic cited department", Citations: []domain.TSCCitation{{SourceID: passage.ID, Quote: quote}}}
	edited, e := service.Edit(ctx, "owner", draft.ID, job.ID, job.Version, answer, "Fixture source correction")
	if e != nil {
		t.Fatal("TSC edit", e)
	}
	if _, e = service.Edit(ctx, "owner", draft.ID, job.ID, job.Version, answer, "Stale correction"); e == nil {
		t.Fatal("stale edit accepted")
	}
	bad := answer
	bad.Citations = []domain.TSCCitation{{SourceID: passage.ID, Quote: "Fabricated evidence that is absent"}}
	if _, e = service.Edit(ctx, "owner", draft.ID, job.ID, edited.Version, bad, "Bad citation fixture"); e == nil {
		t.Fatal("invented evidence accepted")
	}
	data, e := service.Export(ctx, "owner", draft.ID, job.ID)
	if e != nil || len(data) < 1000 {
		t.Fatal("TSC export", e)
	}
	var raw []byte
	pool.QueryRow(ctx, "SELECT result_cipher FROM tender.tsc_jobs WHERE id=$1", job.ID).Scan(&raw)
	if strings.Contains(string(raw), answer.Text) {
		t.Fatal("plaintext answer persisted")
	}
	reviewed := answer
	reviewed.Accepted = true
	accepted, e := service.Edit(ctx, "owner", draft.ID, job.ID, edited.Version, reviewed, "")
	if e != nil {
		t.Fatal("checkbox acceptance without typed reason", e)
	}
	foundAccepted := false
	for _, a := range accepted.Result.Answers {
		if a.QuestionID == answer.QuestionID {
			foundAccepted = a.Accepted
		}
	}
	if !foundAccepted {
		t.Fatal("acceptance did not persist")
	}
	tampered := reviewed
	tampered.Text = "Changed without reopening"
	if _, e = service.Edit(ctx, "owner", draft.ID, job.ID, accepted.Version, tampered, ""); e == nil {
		t.Fatal("accepted answer changed")
	}
	if _, e = service.Research(ctx, "owner", draft.ID, job.ID, answer.QuestionID, "", "gemini", tenderout.Values{}.ID(), accepted.Version); e == nil {
		t.Fatal("accepted answer re-queried")
	}
	reviewed.Accepted = false
	edited, e = service.Edit(ctx, "owner", draft.ID, job.ID, accepted.Version, reviewed, "")
	if e != nil {
		t.Fatal("reopen accepted answer", e)
	}
	var reviewEvents int
	pool.QueryRow(ctx, "SELECT count(*) FROM tender.tsc_events WHERE job_id=$1 AND action IN ('answer_accepted','answer_reopened') AND created_by_id=$2 AND length(reason)>0", job.ID, owner).Scan(&reviewEvents)
	if reviewEvents != 2 {
		t.Fatal("automatic review audit missing", reviewEvents)
	}
	rollback := errors.New("fixture rollback")
	if e = unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, _ ports.DraftRepository) error {
		_, e := r.Edit(ctx, edited, edited.Version, answer, "rollback fixture", owner)
		if e != nil {
			return e
		}
		return rollback
	}); !errors.Is(e, rollback) {
		t.Fatal(e)
	}
	unchanged, e := service.Get(ctx, "owner", draft.ID, job.ID)
	if e != nil || unchanged.Version != edited.Version {
		t.Fatal("answer rollback lost", e)
	}
	next, e := service.Create(ctx, "owner", draft.ID, tenderout.Values{}.ID(), "manual", draft.Version)
	if e != nil {
		t.Fatal(e)
	}
	var first, second *domain.TSCJob
	e = unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, _ ports.DraftRepository) error {
		var e error
		first, e = r.Claim(ctx)
		return e
	})
	if e != nil || first == nil {
		t.Fatal("claim", e)
	}
	execFixture(t, pool, "UPDATE tender.tsc_jobs SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1", next.ID)
	e = unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, _ ports.DraftRepository) error {
		var e error
		second, e = r.Claim(ctx)
		return e
	})
	if e != nil || second == nil || second.ClaimID == first.ClaimID {
		t.Fatal("restart recovery", e)
	}
	if e = unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, _ ports.DraftRepository) error {
		return r.Finish(ctx, *first, job.Result, "")
	}); e == nil {
		t.Fatal("stale worker accepted")
	}
	if e = unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, _ ports.DraftRepository) error {
		return r.Finish(ctx, *second, job.Result, "")
	}); e != nil {
		t.Fatal("current worker rejected", e)
	}
	third, e := service.Create(ctx, "owner", draft.ID, tenderout.Values{}.ID(), "manual", draft.Version)
	if e != nil {
		t.Fatal(e)
	}
	execFixture(t, pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='tenders.tsc.write')", role)
	if _, e = service.Edit(ctx, "owner", draft.ID, job.ID, edited.Version, answer, "Revoked fixture"); !errors.Is(e, domain.Forbidden) {
		t.Fatal("revoked write accepted", e)
	}
	if _, e = service.Once(ctx); e != nil {
		t.Fatal("revoked worker", e)
	}
	blocked, e := service.Get(ctx, "owner", draft.ID, third.ID)
	if e != nil || blocked.State != "failed" || blocked.Error != "source_or_access_changed" {
		t.Fatal("revoked job completed", e, blocked.State)
	}
	execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='tenders.tsc.write'", role)
	execFixture(t, pool, "UPDATE tender.document_drafts SET version=version+1 WHERE id=$1", draft.ID)
	if _, e = service.Export(ctx, "owner", draft.ID, job.ID); e == nil {
		t.Fatal("stale source export allowed")
	}
	var count int
	pool.QueryRow(ctx, "SELECT count(*) FROM tender.tsc_events WHERE job_id=$1 AND result_cipher IS NOT NULL AND action IN ('ready','answer_corrected')", job.ID).Scan(&count)
	if count != 2 {
		t.Fatal("result revision history", count)
	}
	t.Log("TSC native: replay/scope/citations/rollback/encryption/claim fencing/revocation/source staleness passed")
}
