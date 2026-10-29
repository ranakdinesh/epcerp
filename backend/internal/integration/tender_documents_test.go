package integration

import (
	"bytes"
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	authmodule "github.com/ranakdinesh/epcERP/internal/modules/auth"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	authcontracts "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	readout "github.com/ranakdinesh/epcERP/internal/modules/document_reading/adapters/outbound"
	readapp "github.com/ranakdinesh/epcERP/internal/modules/document_reading/application"
	readdomain "github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	readports "github.com/ranakdinesh/epcERP/internal/modules/document_reading/ports"
	eventout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	inbound "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

type fixtureEngine struct{ fail bool }

func (e fixtureEngine) Read(_ context.Context, _ []byte, _ string, hash string) (readdomain.Result, error) {
	if e.fail {
		return readdomain.Result{}, readdomain.Invalid("Unreadable fixture; inspect original")
	}
	return readdomain.Result{Schema: readdomain.ReaderVersion, ReaderVersion: readdomain.ReaderVersion, SourceSHA256: hash, Segments: []readdomain.Segment{{Kind: "page", Label: "Page 1", Text: "Tender source evidence", Method: "native"}}, Warnings: []string{}}, nil
}
func TestTenderDocumentOwnershipQueueAndAtomicJournal(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	actor := "82e66d87-c4db-4771-bbdb-03ad3684e345"
	other := "18e66d87-c4db-4771-bbdb-03ad3684e345"
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,'reader_owner','owner@fixture.invalid','unusable',true,false),($2,'reader_other','other@fixture.invalid','unusable',true,false)", actor, other)
	permission := []string{"tenders.register.view", "tenders.register.read", "tenders.register.write", "tenders.processing.read", "tenders.processing.write"}
	ids := projectIdentities{"owner": auth.Identity{ID: actor, Active: true, Permissions: permission}, "other": auth.Identity{ID: other, Active: true, Permissions: permission}, "reader": auth.Identity{ID: actor, Active: true, Permissions: permission[:2]}, "admin": auth.Identity{ID: actor, Active: true, Superuser: true}, "portfolio": auth.Identity{ID: other, Active: true, Permissions: append(append([]string{}, permission...), "tenders.portfolio.read")}}
	cipher, e := readout.NewCipher(strings.Repeat("ab", 32))
	if e != nil {
		t.Fatal(e)
	}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventout.NewJournal(c)) }
	build := func(j func(database.Connection) events.Writer) *app.Service {
		documents := func(c database.Connection) ports.Documents {
			return out.NewDocuments(readapp.New(readout.NewRepository(c, j(c)), cipher, readout.Values{}, readout.Inspector{}, readout.Codec{}), readout.Inspector{})
		}
		return app.New(out.NewUnitOfWork(pool, documents, j).WithUsers(func(c database.Connection) authcontracts.AssignmentDirectory {
			return (&authmodule.Module{}).Assignments(c)
		}), out.NewAuthBridge(ids), out.Values{})
	}
	service := build(journal)
	pdf := []byte("%PDF-1.7\nfixture")
	if _, e = service.Upload(ctx, "reader", "", "tender.pdf", pdf); !errors.Is(e, domain.Forbidden) {
		t.Fatal("missing processing permission accepted", e)
	}
	if _, e = service.Upload(ctx, "owner", "", "spoof.pdf", []byte("spoof")); e == nil {
		t.Fatal("spoof accepted")
	}
	if _, e = build(func(database.Connection) events.Writer { return failingCategoryJournal{} }).Upload(ctx, "owner", "", "tender.pdf", pdf); e == nil {
		t.Fatal("audit failure committed")
	}
	var count int
	pool.QueryRow(ctx, "SELECT count(*) FROM tender.tender_fms_intakes").Scan(&count)
	if count != 0 {
		t.Fatal("audit rollback")
	}
	var wg sync.WaitGroup
	results := make(chan domain.Intake, 6)
	errs := make(chan error, 6)
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := service.Upload(ctx, "owner", "", "tender.pdf", pdf)
			results <- v
			errs <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal("concurrent dedup", e)
		}
	}
	var first domain.Intake
	for v := range results {
		if first.ID == "" {
			first = v
		}
		if first.ID != v.ID || len(v.Documents) != 1 {
			t.Fatal("duplicate identity")
		}
	}
	doc := first.Documents[0]
	otherRows, e := service.List(ctx, "other")
	if e != nil || len(otherRows) != 0 {
		t.Fatal("owner leak", e)
	}
	if _, e = service.Artifact(ctx, "other", first.ID, doc.ID, "original"); !errors.Is(e, domain.Missing) {
		t.Fatal("source ownership", e)
	}
	if _, e = service.Upload(ctx, "portfolio", first.ID, "extra.pdf", []byte("%PDF-1.7\nextra")); !errors.Is(e, domain.Forbidden) {
		t.Fatal("portfolio read became write", e)
	}
	a, e := service.Artifact(ctx, "owner", first.ID, doc.ID, "original")
	if e != nil || !bytes.Equal(a.Data, pdf) {
		t.Fatal("original roundtrip", e)
	}
	worker := readapp.NewWorker(readout.NewUnitOfWork(pool, journal), cipher, readout.Values{}, fixtureEngine{}, readout.Codec{})
	if worked, e := worker.Once(ctx); !worked || e != nil {
		t.Fatal("worker", e)
	}
	rows, e := service.List(ctx, "owner")
	if e != nil || rows[0].Documents[0].Status != "ready" {
		t.Fatal("ready persistence", e)
	}
	a, e = service.Artifact(ctx, "owner", first.ID, doc.ID, "text")
	if e != nil || !bytes.Contains(a.Data, []byte("Tender source evidence")) {
		t.Fatal("reading roundtrip", e)
	}
	if _, e = pool.Exec(ctx, "UPDATE document_reading_results SET reader_version='tampered' WHERE document_id=$1", doc.ID); e == nil {
		t.Fatal("immutable reading modified")
	}
	if _, e = pool.Exec(ctx, "UPDATE document_readings SET original=decode('aa','hex') WHERE id=$1", doc.ID); e == nil {
		t.Fatal("immutable original modified")
	}
	if _, e = service.Retry(ctx, "owner", first.ID, doc.ID, rows[0].Documents[0].Version); e == nil {
		t.Fatal("ready retried")
	}
	// A failed attempt can be retried with a matching version, up to three attempts.
	bad, e := service.Upload(ctx, "owner", first.ID, "bad.pdf", []byte("%PDF-1.7\nbad"))
	if e != nil {
		t.Fatal(e)
	}
	badDoc := bad.Documents[len(bad.Documents)-1]
	failureWorker := readapp.NewWorker(readout.NewUnitOfWork(pool, journal), cipher, readout.Values{}, fixtureEngine{fail: true}, readout.Codec{})
	for attempt := 1; attempt <= 3; attempt++ {
		if worked, e := failureWorker.Once(ctx); !worked || e != nil {
			t.Fatal("failure attempt", e)
		}
		rows, e = service.List(ctx, "owner")
		if e != nil {
			t.Fatal(e)
		}
		for _, d := range rows[0].Documents {
			if d.ID == badDoc.ID {
				badDoc = d
			}
		}
		if badDoc.Status != "failed" || badDoc.Attempts != int32(attempt) {
			t.Fatal("attempt state", badDoc)
		}
		_, e = service.Retry(ctx, "owner", first.ID, badDoc.ID, badDoc.Version)
		if attempt < 3 && e != nil {
			t.Fatal("retry", e)
		}
		if attempt == 3 && e == nil {
			t.Fatal("exhausted retry accepted")
		}
	}
	// Expired claims are recovered; stale completion cannot replace the new lease.
	leased, e := service.Upload(ctx, "owner", first.ID, "lease.pdf", []byte("%PDF-1.7\nlease"))
	if e != nil {
		t.Fatal(e)
	}
	_ = leased
	u := readout.NewUnitOfWork(pool, journal)
	var old, newJob *readdomain.Document
	claim := func(dst **readdomain.Document) {
		t.Helper()
		if e := u.Within(ctx, func(ctx context.Context, r readports.Repository) error {
			var e error
			*dst, e = r.Claim(ctx, readout.Values{}.ID())
			if e != nil {
				return e
			}
			return r.Event(ctx, **dst, "processing", "")
		}); e != nil {
			t.Fatal(e)
		}
	}
	claim(&old)
	execFixture(t, pool, "UPDATE document_readings SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1", old.ID)
	claim(&newJob)
	if newJob.ID != old.ID || newJob.ClaimID == old.ClaimID {
		t.Fatal("lease recovery")
	}
	if e = u.Within(ctx, func(ctx context.Context, r readports.Repository) error {
		_, e := r.Finish(ctx, *old, nil, "", readdomain.Result{}, "Stale")
		return e
	}); !errors.Is(e, readdomain.Conflict) {
		t.Fatal("stale claim completed", e)
	}
	history, e := service.History(ctx, "owner", first.ID)
	if e != nil || len(history) < 10 {
		t.Fatal("history", len(history), e)
	}
	pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module IN ('tender_fms','document_reading')").Scan(&count)
	if count < 10 {
		t.Fatal("central journal")
	}

	otherIntake, e := service.Upload(ctx, "owner", "", "separate.pdf", []byte("%PDF-1.7\nseparate"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = service.Artifact(ctx, "owner", first.ID, otherIntake.Documents[0].ID, "original"); e == nil {
		t.Fatal("document crossed an authorized intake boundary")
	}
	router := chi.NewRouter()
	inbound.Register(router, service)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	file, _ := mw.CreateFormFile("file", "tender.pdf")
	file.Write(pdf)
	mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/tender-fms/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer owner")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 202 {
		t.Fatal("HTTP upload", response.Code, response.Body.String())
	}
	req = httptest.NewRequest("GET", "/api/v1/tender-fms", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 401 {
		t.Fatal("anonymous list")
	}
	m, e := schema.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Migrate(20); e == nil {
		t.Fatal("populated rollback accepted")
	}
}

func TestTenderNativeDocumentReadingRoundTrip(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	actor := "28e66d87-c4db-4771-bbdb-03ad3684e345"
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,'native_reader','native@fixture.invalid','unusable',true,false)", actor)
	ids := projectIdentities{"admin": auth.Identity{ID: actor, Active: true, Superuser: true}}
	cipher, e := readout.NewCipher(strings.Repeat("ab", 32))
	if e != nil {
		t.Fatal(e)
	}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventout.NewJournal(c)) }
	documents := func(c database.Connection) ports.Documents {
		return out.NewDocuments(readapp.New(readout.NewRepository(c, journal(c)), cipher, readout.Values{}, readout.Inspector{}, readout.Codec{}), readout.Inspector{})
	}
	service := app.New(out.NewUnitOfWork(pool, documents, journal).WithUsers(func(c database.Connection) authcontracts.AssignmentDirectory {
		return (&authmodule.Module{}).Assignments(c)
	}), out.NewAuthBridge(ids), out.Values{})
	data, e := os.ReadFile("../modules/document_reading/adapters/outbound/testdata/native.docx")
	if e != nil {
		t.Fatal(e)
	}
	v, e := service.Upload(ctx, "admin", "", "native.docx", data)
	if e != nil {
		t.Fatal(e)
	}
	worker := readapp.NewWorker(readout.NewUnitOfWork(pool, journal), cipher, readout.Values{}, readout.Parser{}, readout.Codec{})
	if worked, e := worker.Once(ctx); !worked || e != nil {
		t.Fatal("real native reading", e)
	}
	text, e := service.Artifact(ctx, "admin", v.ID, v.Documents[0].ID, "text")
	if e != nil || !bytes.Contains(text.Data, []byte("Tender source document fixture")) || !bytes.Contains(text.Data, []byte("100 meters")) {
		t.Fatal("real paragraph/table reading persistence", e)
	}
	original, e := service.Artifact(ctx, "admin", v.ID, v.Documents[0].ID, "original")
	if e != nil || !bytes.Equal(original.Data, data) {
		t.Fatal("original changed", e)
	}
	var ciphertext []byte
	if e = pool.QueryRow(ctx, "SELECT result FROM document_reading_results WHERE document_id=$1", v.Documents[0].ID).Scan(&ciphertext); e != nil || bytes.Contains(ciphertext, []byte("Tender source document fixture")) {
		t.Fatal("result not encrypted", e)
	}
}
