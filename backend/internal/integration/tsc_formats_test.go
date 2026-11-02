package integration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	tenderhttp "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type formatPreviewStub struct{ calls int }

func (p *formatPreviewStub) Render(_ context.Context, f domain.TSCFormat, pdf bool) ([]byte, error) {
	p.calls++
	if len(f.Questions) == 0 {
		return nil, errors.New("missing saved questions")
	}
	if pdf {
		return []byte("%PDF-1.7 fixture"), nil
	}
	return []byte("PK fixture"), nil
}

func formatUsers(c database.Connection) auth.AssignmentDirectory {
	return authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
}
func TestTSCFormatsNativePermissionsDuplicatesConcurrencyAndHistory(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	v := out.Values{}
	execFixture(t, f.pool, "UPDATE auth.users SET is_superuser=true WHERE id=$1", f.actor)
	ids := projectIdentities{"admin": {ID: f.actor, Active: true}, "other": {ID: f.outsider, Active: true}}
	preview := &formatPreviewStub{}
	service := app.NewTSCFormats(out.NewTSCFormatUnit(f.pool, formatUsers, purchaseJournal), out.NewAuthBridge(ids), v, preview)
	if _, e := service.List(ctx, "other"); !errors.Is(e, domain.Forbidden) {
		t.Fatal("unprivileged master read", e)
	}
	seed, e := service.Get(ctx, "admin", domain.DefaultTSCFormatID)
	if e != nil || len(seed.Questions) != 70 || seed.Name != "EPC Works — Standard TSC" {
		t.Fatal("uploaded format missing", e)
	}
	seedInput := domain.TSCFormatInput{Name: seed.Name, Active: true, Questions: seed.Questions}
	if e := domain.ValidateTSCFormat(&seedInput); e != nil {
		t.Fatal("uploaded question wording is not unique", e)
	}
	cloneInput := domain.TSCFormatInput{Name: "Synthetic uploaded copy", Active: true, Questions: seed.Questions}
	clone, e := service.Save(ctx, "admin", "", cloneInput)
	if e != nil || clone.Questions[0].ID != seed.Questions[0].ID {
		t.Fatal("copy lost reviewed extraction identity", e)
	}
	cloneInput.Version = clone.Version
	cloneInput.Questions = append([]domain.TSCQuestion(nil), clone.Questions...)
	cloneInput.Questions[0].Text = "What is the revised equipment delivery deadline?"
	revised, e := service.Save(ctx, "admin", clone.ID, cloneInput)
	if e != nil || revised.Questions[0].ID == seed.Questions[0].ID {
		t.Fatal("changed question retained unrelated extraction rule", e)
	}
	input := domain.TSCFormatInput{Name: "Synthetic supply tender", Purpose: "Solar", Active: true, Questions: []domain.TSCQuestion{{Section: "Commercial", Text: "What is the delivery period?"}, {Section: "Commercial", Text: "What is the warranty period?"}}}
	created, e := service.Save(ctx, "admin", "", input)
	if e != nil || created.Version != 1 || len(created.Questions) != 2 || created.Purpose != "Solar" || created.CreatedAt.IsZero() {
		t.Fatal("create", e)
	}
	input.Name = "SYNTHETIC SUPPLY TENDER"
	if _, e := service.Save(ctx, "admin", "", input); e == nil {
		t.Fatal("duplicate format name accepted")
	}
	duplicate := domain.TSCFormatInput{Name: "Duplicate question fixture", Active: true, Questions: []domain.TSCQuestion{{Section: "First", Text: "Tender fee?"}, {Section: "Other", Text: " TENDER   FEE : "}}}
	if _, e := service.Save(ctx, "admin", "", duplicate); e == nil {
		t.Fatal("cross-section duplicate accepted")
	}
	if _, e := f.pool.Exec(ctx, `INSERT INTO tender.tsc_format_questions(format_id,id,position,section,printed_number,question)VALUES($1,'bypass',3,'Other','','WHAT IS THE DELIVERY PERIOD:')`, created.ID); e == nil {
		t.Fatal("database duplicate bypass accepted")
	}
	update := domain.TSCFormatInput{Name: created.Name, Purpose: created.Purpose, Active: true, Version: 1, Questions: created.Questions}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := service.Save(ctx, "admin", created.ID, update); results <- e }()
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for e := range results {
		if e == nil {
			successes++
		} else {
			var d *domain.CaptureError
			if errors.As(e, &d) && d.Status == 409 {
				conflicts++
			} else {
				t.Fatal("unexpected concurrent result", e)
			}
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("stale edit was not fenced", successes, conflicts)
	}
	for _, sql := range []string{`UPDATE tender.tsc_format_events SET version=99`, `DELETE FROM tender.tsc_format_events`, `TRUNCATE tender.tsc_format_events`} {
		if _, e := f.pool.Exec(ctx, sql); e == nil {
			t.Fatal("mutable format history")
		}
	}
	stored, _ := service.Get(ctx, "admin", created.ID)
	failed := app.NewTSCFormats(out.NewTSCFormatUnit(f.pool, formatUsers, func(database.Connection) events.Writer { return purchaseRejectJournal{} }), out.NewAuthBridge(ids), v, nil)
	update.Version = stored.Version
	update.Name = "Must roll back with event"
	if _, e := failed.Save(ctx, "admin", created.ID, update); e == nil {
		t.Fatal("journal failure ignored")
	}
	unchanged, _ := service.Get(ctx, "admin", created.ID)
	if unchanged.Name != stored.Name || unchanged.Version != stored.Version {
		t.Fatal("failed audit changed format")
	}
	router := chi.NewRouter()
	tenderhttp.RegisterTSCFormats(router, service)
	for _, tc := range []struct {
		method, path, token, body string
		status                    int
	}{{"GET", "", "admin", "", 200}, {"GET", "/" + created.ID, "admin", "", 200}, {"GET", "", "other", "", 403}, {"GET", "/" + created.ID + "/preview.pdf", "other", "", 403}, {"GET", "/" + created.ID + "/template.docx", "other", "", 403}, {"GET", "/" + created.ID + "/preview.pdf", "admin", "", 200}, {"GET", "/" + created.ID + "/template.docx", "admin", "", 200}, {"POST", "", "other", `{"name":"Forged","is_active":true,"questions":[]}`, 403}, {"PATCH", "/" + created.ID, "admin", `{"expected_version":1,"name":"stale","questions":[]}`, 409}, {"POST", "", "admin", `{"name":"Unknown field","approve":true}`, 422}} {
		req := httptest.NewRequest(tc.method, "/api/v1/settings/tsc-formats"+tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+tc.token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("format API %s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	if preview.calls != 2 {
		t.Fatal("preview rendered before authorization", preview.calls)
	}
	if unchanged.CreatedAt != created.CreatedAt || unchanged.Purpose != "Solar" {
		t.Fatal("saved metadata changed unexpectedly")
	}

	execFixture(t, f.pool, "UPDATE auth.users SET is_active=false WHERE id=$1", f.actor)
	if _, e := service.List(ctx, "admin"); !errors.Is(e, domain.Forbidden) {
		t.Fatal("inactive actor retained master permission", e)
	}
}

// Uses the already-read synthetic source fixture; no provider requests are made.
func verifyNamedTSCFormats(t *testing.T, ctx context.Context, pool *pgxpool.Pool, service *app.TSC, ids projectIdentities, journal func(database.Connection) events.Writer, draft domain.DocumentDraft, owner, role string) {
	t.Helper()
	for _, code := range []string{"settings.tsc_formats.read", "settings.tsc_formats.write"} {
		execFixture(t, pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture') ON CONFLICT(code)DO NOTHING", out.Values{}.ID(), code)
		execFixture(t, pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	formats := app.NewTSCFormats(out.NewTSCFormatUnit(pool, formatUsers, journal), out.NewAuthBridge(ids), out.Values{}, nil)
	input := domain.TSCFormatInput{Name: "Synthetic equipment supply TSC", Active: true, Questions: []domain.TSCQuestion{{Section: "Delivery", Text: "What is the equipment delivery period?"}}}
	format, e := formats.Save(ctx, "owner", "", input)
	if e != nil {
		t.Fatal("named format", e)
	}
	key := out.Values{}.ID()
	selection := domain.TSCFormatSelection{ID: format.ID, Version: format.Version}
	job, e := service.Create(ctx, "owner", draft.ID, key, "manual", draft.Version, selection)
	if e != nil {
		t.Fatal("selected format generation", e)
	}
	if _, e = service.Once(ctx); e != nil {
		t.Fatal(e)
	}
	job, e = service.Get(ctx, "owner", draft.ID, job.ID)
	if e != nil || len(job.Input.Questions) != 1 || job.Input.FormatName != format.Name || job.Input.FormatVersion != 1 {
		t.Fatal("format snapshot", e)
	}
	before, _ := json.Marshal(job.Input)
	input.Version = 1
	input.Name = "Renamed equipment TSC"
	input.Questions = append(format.Questions, domain.TSCQuestion{Section: "Warranty", Text: "What is the warranty term?"})
	changed, e := formats.Save(ctx, "owner", format.ID, input)
	if e != nil {
		t.Fatal("format revision", e)
	}
	old, e := service.Get(ctx, "owner", draft.ID, job.ID)
	after, _ := json.Marshal(old.Input)
	if e != nil || string(before) != string(after) {
		t.Fatal("master edit rewrote historical questions", e)
	}
	if _, e = service.Export(ctx, "owner", draft.ID, job.ID); e != nil {
		t.Fatal("custom-length DOCX export", e)
	}
	replay, e := service.Create(ctx, "owner", draft.ID, key, "manual", draft.Version, selection)
	if e != nil || replay.ID != job.ID {
		t.Fatal("replay lost original format revision", e)
	}
	if _, e = service.Create(ctx, "owner", draft.ID, out.Values{}.ID(), "manual", draft.Version, selection); e == nil {
		t.Fatal("stale format selection accepted")
	}
	input.Version = changed.Version
	input.Active = false
	input.Questions = changed.Questions
	if _, e = formats.Save(ctx, "owner", format.ID, input); e != nil {
		t.Fatal(e)
	}
	if _, e = service.Create(ctx, "owner", draft.ID, out.Values{}.ID(), "manual", draft.Version, domain.TSCFormatSelection{ID: format.ID, Version: changed.Version + 1}); e == nil {
		t.Fatal("inactive format used for new job")
	}
	if _, e = pool.Exec(ctx, "UPDATE tender.tsc_jobs SET format_version=99 WHERE id=$1", job.ID); e == nil {
		t.Fatal("format snapshot identity mutable")
	}
}
