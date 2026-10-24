package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	authOut "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authApp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventOut "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventApp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"

	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	projectIn "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/inbound"
	projectOut "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	projectApp "github.com/ranakdinesh/epcERP/internal/modules/projects/application"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	calendarOut "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/adapters/outbound"
	calendarApp "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/application"
	calendar "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/contracts"
	"net/http/httptest"
	"testing"
)

func TestProjectMetadataCorrectionScopeAuditCalculationAndRollback(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	values := projectOut.SystemValues{}
	admin, client := values.ID(), values.ID()
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'editor','editor@example.invalid','fixture',true,true)", admin)
	execFixture(t, pool, `INSERT INTO clients(id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,legacy_source,legacy_source_id,is_active) VALUES($1,'EDIT','Edit client','edit client','Edit client','','','','','','','','','','','','','India','','',true)`, client)
	ids := projectIdentities{"admin": {ID: admin, Active: true, Superuser: true}, "owner": {ID: admin, Active: true, Permissions: []string{"projects.master.read", "projects.master.write"}}, "unrelated": {ID: values.ID(), Active: true, Permissions: []string{"projects.master.read", "projects.master.write", "projects.manager_assignments.write"}}, "assignment-only": {ID: admin, Active: true, Permissions: []string{"projects.master.read", "projects.manager_assignments.write"}}}
	reader := func(c database.Connection) calendar.Reader {
		return calendarApp.Reader{Repo: calendarOut.Repository{Conn: c}}
	}
	projects := projectApp.New(projectOut.NewUnitOfWork(pool, func(c database.Connection) masterContracts.ClientDirectory {
		return masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c))
	}, func(c database.Connection) auth.AssignmentDirectory {
		return authApp.NewAssignmentDirectory(authOut.NewAssignmentReader(c))
	}, func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) }, reader).WithPurchasing(projectPurchaseOwnership), projectOut.NewAuthBridge(ids), values)
	p, e := projects.Create(ctx, "admin", domain.Create{Name: "Editable draft", ClientID: client, ManagerID: admin, ProcurementExecutiveID: admin, Capture: domain.Capture{ProjectType: "electrical", SiteState: "Delhi", SiteCity: "New Delhi", ContractValue: "100.00", Currency: "INR", LOIDate: "2026-10-01", ScheduleStartBasis: "loi_date", PlannedCompletionDate: "2026-10-31", PlannedDurationDays: "31"}})
	if e != nil {
		t.Fatal(e)
	}
	metadata := projects.Metadata()
	correction := domain.Correction{Name: "Corrected draft", ClientID: client, Reason: "Correct scope and actual start", ExpectedVersion: p.Version, Capture: p.Capture}
	correction.ActualStartDate = "2026-10-05"
	correction.SafetyBufferPercent = "20"
	correction.ContractValue = "99999999999999.99"
	for _, token := range []string{"unrelated", "assignment-only"} {
		if _, e := metadata.Correct(ctx, token, p.ID, correction); !errors.Is(e, domain.ErrForbidden) {
			t.Fatal("scope denial", e)
		}
		if _, e := metadata.Options(ctx, token, p.ID); !errors.Is(e, domain.ErrForbidden) {
			t.Fatal("options scope denial", e)
		}
	}
	if _, e := metadata.Options(ctx, "owner", p.ID); e != nil {
		t.Fatal("scoped owner options", e)
	}
	saved, e := metadata.Correct(ctx, "owner", p.ID, correction)
	if e != nil {
		t.Fatal(e)
	}
	if saved.Name != correction.Name || saved.Version != 2 || saved.ContractValue != correction.ContractValue || saved.PlannedDurationDays != "22" || saved.SafetyBufferDays != "17" || saved.ManagerID != admin || saved.ProcurementExecutiveID != admin {
		t.Fatal("roundtrip/derived totals/ownership")
	}
	preview, e := metadata.Preview(ctx, "owner", p.ID, correction.Capture)
	if e != nil || *preview.CompletionDays != 22 {
		t.Fatal("owner scoped preview", e)
	}
	if _, e := metadata.Correct(ctx, "owner", p.ID, correction); !errors.Is(e, domain.ErrStale) {
		t.Fatal("stale overwrite", e)
	}
	history, e := metadata.History(ctx, "owner", p.ID)
	if e != nil || len(history) != 1 || history[0].Before.Name != p.Name || history[0].After.Name != saved.Name || history[0].ActorID != admin || history[0].Reason != correction.Reason {
		t.Fatal("permanent before/after", e)
	}
	for _, q := range []string{"UPDATE project_metadata_changes SET reason=reason", "DELETE FROM project_metadata_changes", "TRUNCATE project_metadata_changes"} {
		if _, e := pool.Exec(ctx, q); e == nil {
			t.Fatal("mutable correction history")
		}
	}
	execFixture(t, pool, `CREATE FUNCTION reject_metadata_fixture() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END $$; CREATE CONSTRAINT TRIGGER reject_metadata_fixture AFTER INSERT ON project_metadata_changes DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_metadata_fixture()`)
	correction.ExpectedVersion = saved.Version
	correction.Name = "Must roll back"
	if _, e := metadata.Correct(ctx, "owner", p.ID, correction); e == nil {
		t.Fatal("commit failure accepted")
	}
	loaded, e := projects.Get(ctx, "admin", p.ID)
	if e != nil || loaded.Name != saved.Name || loaded.Version != saved.Version {
		t.Fatal("failed update leaked", e)
	}
	history, e = metadata.History(ctx, "owner", p.ID)
	if e != nil || len(history) != 1 {
		t.Fatal("failed history leaked", e)
	}
	execFixture(t, pool, "DROP TRIGGER reject_metadata_fixture ON project_metadata_changes; DROP FUNCTION reject_metadata_fixture()")
	// Incomplete draft edits stay pending, without weakening normal creation.
	correction.Name = saved.Name
	correction.Capture = domain.Capture{Currency: "INR", ProjectType: "electrical"}
	pending, e := metadata.Correct(ctx, "owner", p.ID, correction)
	if e != nil || pending.ContractValue != "" || pending.Calculation.CompletionDays != nil {
		t.Fatal("pending draft correction", e)
	}
	router := chi.NewRouter()
	projectIn.RegisterMetadata(router, metadata)
	body := map[string]any{"name": pending.Name, "client_id": client, "reason": "Unsupported assignment overwrite", "expected_version": pending.Version, "project_manager_id": admin}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest("PATCH", "/api/v1/projects/"+p.ID, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 422 {
		t.Fatal("metadata accepted ownership fields", w.Code)
	}
	execFixture(t, pool, "UPDATE projects SET lifecycle_status='closed' WHERE id=$1", p.ID)
	correction.ExpectedVersion = pending.Version
	if _, e := metadata.Correct(ctx, "admin", p.ID, correction); e == nil {
		t.Fatal("non-draft editable")
	}
	m, e := schema.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Migrate(25); e == nil {
		t.Fatal("populated edit history rollback allowed")
	}
}
