package integration

import (
	"bytes"
	"context"
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
	projectOut "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	projectApp "github.com/ranakdinesh/epcERP/internal/modules/projects/application"
	project "github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	calendarIn "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/adapters/inbound"
	calendarOut "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/adapters/outbound"
	calendarApp "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/application"
	calendar "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/contracts"
	holiday "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/domain"
	"net/http/httptest"
	"testing"
)

func TestCalendarRecalculatesProjectsAtomically(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	values := projectOut.SystemValues{}
	admin, client := values.ID(), values.ID()
	execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,'calendar_admin','calendar@example.invalid','fixture',true,true)", admin)
	execFixture(t, pool, `INSERT INTO clients(id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,legacy_source,legacy_source_id,is_active) VALUES($1,'CAL','Calendar fixture','calendar fixture','Calendar fixture','','','','','','','','','','','','','India','','',true)`, client)
	identities := projectIdentities{"admin": {ID: admin, Active: true, Superuser: true}, "reader": {ID: admin, Active: true, Permissions: []string{"settings.holidays.read"}}}
	reader := func(c database.Connection) calendar.Reader {
		return calendarApp.Reader{Repo: calendarOut.Repository{Conn: c}}
	}
	writer := func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) }
	projects := projectApp.New(projectOut.NewUnitOfWork(pool, func(c database.Connection) masterContracts.ClientDirectory {
		return masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c))
	}, func(c database.Connection) auth.AssignmentDirectory {
		return authApp.NewAssignmentDirectory(authOut.NewAssignmentReader(c))
	}, writer, reader).WithPurchasing(projectPurchaseOwnership), projectOut.NewAuthBridge(identities), values)
	manager := database.NewManager(pool)
	unit := calendarOut.Unit{Manager: manager, Events: writer, Refresh: func(c database.Connection) func(context.Context, string) (int, error) {
		return projectOut.CalendarUpdater{Conn: c, Events: writer(c), Reader: reader}.Recalculate
	}}
	holidays := calendarApp.Service{Unit: unit, Auth: calendarOut.Auth{Resolver: identities}}
	capture := project.Capture{ProjectType: "electrical", SiteState: "Delhi", SiteCity: "New Delhi", OperationsManagerID: admin, ContractValue: "100", Currency: "INR", LOIDate: "2026-10-01", ScheduleStartBasis: "loi_date", PlannedStartDate: "2026-10-03", PlannedCompletionDate: "2026-10-31", ActualStartDate: "2026-10-05", ActualEndDate: "2026-10-30", SafetyBufferPercent: "20", SafetyBufferDays: "999"}
	preview, e := projects.Preview(ctx, "admin", capture)
	if e != nil || *preview.ExecutionDays != 17 {
		t.Fatal("preview", e)
	}
	p, e := projects.Create(ctx, "admin", project.Create{ProcurementExecutiveID: admin, Name: "Calendar project", ClientID: client, ManagerID: admin, Capture: capture})
	if e != nil {
		t.Fatal(e)
	}
	if p.PlannedDurationDays != "22" || p.SafetyBufferDays != "17" || *p.Calculation.ContractualStartGap != 4 || *p.Calculation.PlannedStartGap != 2 {
		t.Fatal("stored calculation differs")
	}
	h, e := holidays.Save(ctx, "admin", holiday.Change{Name: "Working holiday", Date: "2026-10-06", Active: true, Reason: "Annual calendar"})
	if e != nil {
		t.Fatal(e)
	}
	check := func(days, buffer string, version int32) {
		t.Helper()
		v, e := projects.Get(ctx, "admin", p.ID)
		if e != nil || v.PlannedDurationDays != days || v.SafetyBufferDays != buffer || v.Version != version {
			t.Fatal("calendar refresh", e, v)
		}
	}
	check("21", "16", 2)
	if _, e = holidays.Save(ctx, "admin", holiday.Change{Name: "Weekend holiday", Date: "2026-10-11", Active: true, Reason: "No double subtraction"}); e != nil {
		t.Fatal(e)
	}
	check("21", "16", 3)
	if _, e = holidays.Save(ctx, "admin", holiday.Change{ID: h.ID, Name: h.Name, Date: h.Date, Version: h.Version, Active: false, Reason: "Deactivate"}); e != nil {
		t.Fatal(e)
	}
	check("22", "17", 4)
	if _, e = holidays.Save(ctx, "admin", holiday.Change{ID: h.ID, Name: h.Name, Date: h.Date, Version: h.Version, Active: true, Reason: "Stale"}); !errors.Is(e, holiday.Conflict) {
		t.Fatal("stale accepted", e)
	}
	if _, e = holidays.Save(ctx, "reader", holiday.Change{Name: "Forbidden", Date: "2026-10-08", Active: true, Reason: "Denied"}); !errors.Is(e, holiday.Forbidden) {
		t.Fatal("read gained write", e)
	}
	if _, e = holidays.Save(ctx, "admin", holiday.Change{Name: "Duplicate", Date: h.Date, Active: true, Reason: "Duplicate"}); !errors.Is(e, holiday.Conflict) {
		t.Fatal("duplicate accepted", e)
	}
	var matched int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM project_schedule_changes h JOIN business_events e ON e.id=h.id AND e.entity_version=h.project_version AND e.actor_id=h.created_by_id AND e.scope_id=h.project_id`).Scan(&matched); e != nil || matched != 3 {
		t.Fatal("schedule audit incomplete", e)
	}
	for _, q := range []string{"UPDATE project_schedule_changes SET after_calculation='{}'", "DELETE FROM project_schedule_changes", "TRUNCATE project_schedule_changes", "UPDATE work_calendar_changes SET reason='changed'", "DELETE FROM work_calendar_changes", "TRUNCATE work_calendar_changes"} {
		if _, e = pool.Exec(ctx, q); e == nil {
			t.Fatal("mutable history")
		}
	}
	execFixture(t, pool, `CREATE FUNCTION calendar_fixture_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END $$; CREATE CONSTRAINT TRIGGER calendar_fixture_failure AFTER INSERT ON project_schedule_changes DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION calendar_fixture_failure();`)
	if _, e = holidays.Save(ctx, "admin", holiday.Change{Name: "Rollback", Date: "2026-10-08", Active: true, Reason: "Must roll back"}); e == nil {
		t.Fatal("commit failure accepted")
	}
	check("22", "17", 4)
	if countRows(t, pool, "work_calendar_holidays") != 2 || countRows(t, pool, "work_calendar_changes") != 3 || countRows(t, pool, "project_schedule_changes") != 3 || countRows(t, pool, "business_events") != 8 {
		t.Fatal("transaction leaked records")
	}
	router := chi.NewRouter()
	calendarIn.Register(router, holidays)
	for _, tc := range []struct {
		method, path, token, body string
		status                    int
	}{{"GET", "/api/v1/work-calendar/holidays?year=2026", "admin", "", 200}, {"GET", "/api/v1/work-calendar/holidays?unknown=1", "admin", "", 422}, {"GET", "/api/v1/work-calendar/holidays", "", "", 401}, {"POST", "/api/v1/work-calendar/holidays", "reader", `{"name":"Denied","holiday_date":"2026-10-12","is_active":true,"reason":"Denied"}`, 403}, {"POST", "/api/v1/work-calendar/holidays", "admin", `{"name":"Missing active","holiday_date":"2026-10-12","reason":"Invalid"}`, 422}} {
		r := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		r.Header.Set("Authorization", "Bearer "+tc.token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatal("calendar HTTP", tc.path, w.Code)
		}
	}
	m, e := schema.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Migrate(23); e == nil {
		t.Fatal("populated calendar rollback allowed")
	}
}
