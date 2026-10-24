package integration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	in "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/application"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/domain"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	directorout "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound"
	directorapp "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/application"
	directordomain "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestCommunicationInboxNativeAtomicRecipientReplayAndReceipts(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := t.Context()
	v := out.Values{}
	manager := database.NewManager(f.pool)
	execFixture(t, f.pool, "UPDATE auth.users SET is_superuser=true WHERE id=$1", f.manager)
	codes := []string{"communication.inbox.read", "communication.inbox.write"}
	role := v.ID()
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,'inbox fixture','test',true)", role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'fixture')", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	for _, user := range []string{f.actor, f.outsider} {
		execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", user, role)
	}
	ids := projectIdentities{"admin": auth.Identity{ID: f.manager, Active: true, Superuser: true}, "director": auth.Identity{ID: f.actor, Active: true, Permissions: codes}, "other": auth.Identity{ID: f.outsider, Active: true, Permissions: codes}, "spoof": auth.Identity{ID: f.outsider, Active: true, Superuser: true}}
	governance := directorapp.New(directorout.NewUnitOfWork(f.pool, directorUsers, purchaseJournal), directorout.NewAuthBridge(ids), directorout.Values{})
	service := func(j func(database.Connection) events.Writer) *app.Service {
		return app.New(out.NewUnitOfWork(f.pool, directorUsers, j), out.NewAuthBridge(ids), v)
	}
	s := service(purchaseJournal)
	source := func() events.Event {
		id := v.ID()
		return events.Event{ID: id, Module: "finance_approvals", EntityType: "financial_decision", EntityID: v.ID(), EntityVersion: 1, Action: "approved", ActorID: f.manager, ActorKind: "user", CorrelationID: id, ScopeKind: "project", ScopeID: f.project, Visibility: "restricted", OccurredAt: v.Now()}
	}
	deliver := func(e events.Event, record bool, j func(database.Connection) events.Writer) (domain.Notice, error) {
		var n domain.Notice
		err := manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
			if record {
				if err := purchaseJournal(c).Record(ctx, e, nil); err != nil {
					return err
				}
			}
			var err error
			n, err = out.NewWriter(c, directorout.NewDirectory(c, directorUsers(c)), j(c), v).DirectorDecision(ctx, e.ID)
			return err
		})
		if err != nil {
			n = domain.Notice{}
		}
		return n, err
	}
	missing := source()
	if _, err := deliver(missing, true, purchaseJournal); !errors.Is(err, domain.NoDirector) {
		t.Fatal("missing Director", err)
	}
	var count int
	if e := f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE id=$1", missing.ID).Scan(&count); e != nil || count != 0 {
		t.Fatal("source event survived missing recipient", count, e)
	}
	designation, e := governance.Save(ctx, "admin", directordomain.Input{UserID: f.actor, Active: true, Reason: "Initial notice recipient"})
	if e != nil {
		t.Fatal(e)
	}
	rejected := source()
	if _, e = deliver(rejected, true, func(database.Connection) events.Writer { return purchaseRejectJournal{} }); e == nil {
		t.Fatal("notice journal failure ignored")
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE id=$1", rejected.ID).Scan(&count); e != nil || count != 0 {
		t.Fatal("decision not rolled back", count, e)
	}
	if _, e = out.NewWriter(f.pool, directorout.NewDirectory(f.pool, directorUsers(f.pool)), purchaseJournal(f.pool), v).DirectorDecision(ctx, v.ID()); e == nil {
		t.Fatal("pool writer permitted")
	}
	event := source()
	notice, e := deliver(event, true, purchaseJournal)
	if e != nil || notice.RecipientID != f.actor || notice.DirectorID != designation.Record.ID || notice.DirectorVersion != 1 || notice.Version != 1 {
		t.Fatal("notice", notice, e)
	}
	var wrongContext domain.Notice
	e = manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		var err error
		wrongContext, err = out.NewWriter(c, directorout.NewDirectory(c, directorUsers(c)), purchaseJournal(c), v).DirectorDecision(context.Background(), event.ID)
		return err
	})
	if e == nil || wrongContext.ID != "" {
		t.Fatal("context mismatch permitted", e)
	}
	// Repeated concurrent consumption must retain one notice and one delivery event.
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, e := deliver(event, false, purchaseJournal)
			if e == nil && n.ID != notice.ID {
				e = errors.New("replay returned another notice")
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM communication_notice_events WHERE notice_id=$1", notice.ID).Scan(&count); e != nil || count != 1 {
		t.Fatal("replay duplicated history", count, e)
	}
	if _, e = governance.Save(ctx, "admin", directordomain.Input{UserID: f.outsider, Active: true, Version: 1, Reason: "Director handover"}); e != nil {
		t.Fatal(e)
	}

	// Race the first delivery, not only subsequent replay. One source gets one notice.
	raced := source()
	if e = manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return purchaseJournal(c).Record(ctx, raced, nil)
	}); e != nil {
		t.Fatal(e)
	}
	errs = make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := deliver(raced, false, purchaseJournal); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal("initial delivery race", e)
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM communication_notices WHERE source_event_id=$1", raced.ID).Scan(&count); e != nil || count != 1 {
		t.Fatal("initial duplicate notices", count, e)
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM communication_notice_events e JOIN communication_notices n ON n.id=e.notice_id WHERE n.source_event_id=$1", raced.ID).Scan(&count); e != nil || count != 1 {
		t.Fatal("initial duplicate delivery evidence", count, e)
	}
	// Invalid source module cannot use the trusted financial writer.
	invalidSource := source()
	invalidSource.Module = "purchase_bo"
	if _, e = deliver(invalidSource, true, purchaseJournal); !errors.Is(e, domain.SourceConflict) {
		t.Fatal("nonfinancial event accepted", e)
	}
	replay, e := deliver(event, false, purchaseJournal)
	if e != nil || replay.ID != notice.ID || replay.RecipientID != f.actor || replay.DirectorVersion != 1 {
		t.Fatal("historical recipient moved", replay, e)
	}
	second := source()
	newNotice, e := deliver(second, true, purchaseJournal)
	if e != nil || newNotice.RecipientID != f.outsider || newNotice.DirectorVersion != 2 {
		t.Fatal("new effective recipient", newNotice, e)
	}

	execFixture(t, f.pool, "UPDATE auth.users SET is_active=false WHERE id=$1", f.outsider)
	if _, e = deliver(source(), true, purchaseJournal); !errors.Is(e, domain.NoDirector) {
		t.Fatal("inactive Director received decision", e)
	}
	execFixture(t, f.pool, "UPDATE auth.users SET is_active=true WHERE id=$1", f.outsider)
	if _, e = governance.Save(ctx, "admin", directordomain.Input{UserID: f.outsider, Active: false, Version: 2, Reason: "Revoke Director"}); e != nil {
		t.Fatal(e)
	}
	if replay, e = deliver(event, false, purchaseJournal); e != nil || replay.ID != notice.ID {
		t.Fatal("replay required new designation", e)
	}
	if _, e = deliver(source(), true, purchaseJournal); !errors.Is(e, domain.NoDirector) {
		t.Fatal("revoked designation accepted", e)
	}
	if _, e = s.Read(ctx, "other", notice.ID); !errors.Is(e, domain.Missing) {
		t.Fatal("cross-user disclosure", e)
	}
	if _, e = s.Read(ctx, "admin", notice.ID); !errors.Is(e, domain.Missing) {
		t.Fatal("superuser read another inbox", e)
	}
	if _, e = s.Receipt(ctx, "other", notice.ID, 1, "read"); !errors.Is(e, domain.Missing) {
		t.Fatal("cross-user receipt", e)
	}
	listing, e := s.List(ctx, "director", domain.Query{Page: 1, Size: 20, Status: "unread"})
	if e != nil || listing.Total != 1 || len(listing.Items) != 1 || !listing.CanWrite {
		t.Fatal("personal listing", listing, e)
	}
	w, e := s.Read(ctx, "director", notice.ID)
	if e != nil || w.Notice.ReadAt != nil || len(w.History) != 1 {
		t.Fatal("GET caused receipt", w, e)
	}
	if w, e = service(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Receipt(ctx, "director", notice.ID, 1, "read"); e == nil || w.Notice.ID != "" {
		t.Fatal("receipt journal failure", w, e)
	}
	w, e = s.Read(ctx, "director", notice.ID)
	if e != nil || w.Notice.Version != 1 || w.Notice.ReadAt != nil {
		t.Fatal("receipt rollback", w, e)
	}
	// Serialize optimistic concurrent recipient receipts: one success, one stale.
	errs = make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.Receipt(ctx, "director", notice.ID, 1, "read"); errs <- e }()
	}
	wg.Wait()
	close(errs)
	success, stale := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else if errors.Is(e, domain.Stale) {
			stale++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatal("receipt race", success, stale)
	}

	execFixture(t, f.pool, "CREATE FUNCTION reject_inbox_fixture_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'inbox fixture commit rejected';END $$")
	execFixture(t, f.pool, "CREATE CONSTRAINT TRIGGER reject_inbox_fixture_commit AFTER UPDATE ON communication_notices DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_inbox_fixture_commit()")
	rejectedWorkspace, commitErr := s.Receipt(ctx, "director", notice.ID, 2, "acknowledge")
	var commitPG *pgconn.PgError
	if !errors.As(commitErr, &commitPG) || commitPG.Code != "P0001" || commitPG.Message != "inbox fixture commit rejected" || rejectedWorkspace.Notice.ID != "" {
		t.Fatal("commit failure leaked uncommitted result", commitErr, rejectedWorkspace)
	}
	execFixture(t, f.pool, "DROP TRIGGER reject_inbox_fixture_commit ON communication_notices")
	execFixture(t, f.pool, "DROP FUNCTION reject_inbox_fixture_commit()")
	committed, e := s.Read(ctx, "director", notice.ID)
	if e != nil || committed.Notice.Version != 2 || committed.Notice.AcknowledgedAt != nil || len(committed.History) != 2 {
		t.Fatal("deferred commit rollback", committed, e)
	}
	w, e = s.Receipt(ctx, "director", notice.ID, 2, "acknowledge")
	if e != nil || w.Notice.Version != 3 || w.Notice.ReadAt == nil || w.Notice.AcknowledgedAt == nil || len(w.History) != 3 || w.Notice.UpdatedBy == nil || *w.Notice.UpdatedBy != f.actor {
		t.Fatal("acknowledgement", w, e)
	}
	repeated, e := s.Receipt(ctx, "director", notice.ID, 3, "acknowledge")
	if e != nil || repeated.Notice.Version != 3 || len(repeated.History) != 3 {
		t.Fatal("idempotent current receipt", repeated, e)
	}
	for _, status := range []string{"read", "unread"} {
		list, e := s.List(ctx, "director", domain.Query{Page: 1, Size: 20, Status: status})
		if e != nil || list.Total != 0 {
			t.Fatal("status counts", list, e)
		}
	}
	list, e := s.List(ctx, "director", domain.Query{Page: 1, Size: 20, Status: "acknowledged"})
	if e != nil || list.Total != 1 {
		t.Fatal(list, e)
	}
	for _, query := range []string{"UPDATE communication_notices SET recipient_id=$2 WHERE id=$1", "UPDATE communication_notices SET read_at=NULL,version=version+1 WHERE id=$1", "DELETE FROM communication_notices WHERE id=$1", "DELETE FROM communication_notice_events WHERE notice_id=$1"} {
		args := []any{notice.ID}
		if strings.Contains(query, "$2") {
			args = append(args, f.outsider)
		}
		if _, e = f.pool.Exec(ctx, query, args...); e == nil {
			t.Fatal("SQL evidence rewritten", query)
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='communication_inbox'").Scan(&count); e != nil || count != 5 {
		t.Fatal("central/local receipts reconcile", count, e)
	}
	// Native HTTP rejects invented scope, receipt fields and unbounded queries.
	router := chi.NewRouter()
	in.Register(router, s)
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r := httptest.NewRecorder()
		router.ServeHTTP(r, req)
		return r
	}
	if r := request(http.MethodGet, "/api/v1/inbox", "", "director"); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := request(http.MethodGet, "/api/v1/inbox/"+notice.ID, "", "other"); r.Code != 404 {
		t.Fatal("HTTP cross-user", r.Code)
	}
	if r := request(http.MethodGet, "/api/v1/inbox", "", ""); r.Code != 401 {
		t.Fatal("HTTP authentication", r.Code)
	}
	for _, path := range []string{"/api/v1/inbox?recipient_id=" + f.actor, "/api/v1/inbox?page=1.2", "/api/v1/inbox?size=1000", "/api/v1/inbox?status=resolved", "/api/v1/inbox?status=read&status=unread"} {
		if r := request(http.MethodGet, path, "", "director"); r.Code != 422 {
			t.Fatal("HTTP query", path, r.Code)
		}
	}
	for _, body := range []string{`{}`, `{"version":null}`, `{"version":1.2}`, `{"version":3,"recipient_id":"` + f.actor + `"}`, `{"version":3} {}`, `[]`} {
		if r := request(http.MethodPost, "/api/v1/inbox/"+notice.ID+"/read", body, "director"); r.Code != 422 {
			t.Fatal("HTTP strict receipt", body, r.Code)
		}
	}
	if r := request(http.MethodPost, "/api/v1/inbox/"+notice.ID+"/read", strings.Repeat(" ", 4097), "director"); r.Code != 413 {
		t.Fatal("body size", r.Code)
	}
	if r := request(http.MethodPost, "/api/v1/inbox/"+notice.ID+"/read", `{"version":1}`, "director"); r.Code != 409 {
		t.Fatal("HTTP stale", r.Code)
	}
	if r := request(http.MethodPost, "/api/v1/inbox/"+notice.ID+"/acknowledge", `{"version":3}`, "director"); r.Code != 200 {
		t.Fatal("HTTP receipt", r.Code, r.Body.String())
	} else {
		var out domain.Workspace
		if e = json.Unmarshal(r.Body.Bytes(), &out); e != nil || out.Notice.Version != 3 {
			t.Fatal(e)
		}
	}

	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id IN(SELECT id FROM auth.permissions WHERE code='communication.inbox.write')", role)
	readonly, e := s.Read(ctx, "director", notice.ID)
	if e != nil || readonly.CanWrite {
		t.Fatal("live receipt permission", readonly, e)
	}
	if _, e = s.Receipt(ctx, "director", notice.ID, 3, "read"); !errors.Is(e, domain.Forbidden) {
		t.Fatal("revoked write grant trusted", e)
	}
	if _, e = s.Receipt(ctx, "spoof", newNotice.ID, 1, "read"); !errors.Is(e, domain.Forbidden) {
		t.Fatal("spoofed superuser flag trusted", e)
	}
	execFixture(t, f.pool, "DELETE FROM auth.user_roles WHERE user_id=$1", f.actor)
	if _, e = s.List(ctx, "director", domain.Query{Page: 1, Size: 20}); !errors.Is(e, domain.Forbidden) {
		t.Fatal("stale token grant trusted", e)
	}
	if _, e = s.Read(ctx, "director", notice.ID); !errors.Is(e, domain.Forbidden) {
		t.Fatal("revoked read trusted", e)
	}
	execFixture(t, f.pool, "UPDATE auth.users SET is_active=false WHERE id=$1", f.outsider)
	if _, e = s.List(ctx, "other", domain.Query{Page: 1, Size: 20}); !errors.Is(e, domain.Unauthorized) {
		t.Fatal("inactive recipient allowed", e)
	}
	m, e := schema.Open(ctx, f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Steps(-(schema.LatestVersion - 39)); e == nil {
		t.Fatal("populated notice rollback permitted")
	}
}
