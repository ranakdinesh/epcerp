package integration

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	out "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	app "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	eventmodule "github.com/ranakdinesh/epcERP/internal/modules/event_delivery"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

func TestAuthOrganisationHierarchyAndAudit(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	v, e := security.NewTokens(security.TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "fixture", Audience: "fixture", AccessLifetime: 15 * time.Minute})
	if e != nil {
		t.Fatal(e)
	}
	ids := []string{v.ID(), v.ID(), v.ID(), v.ID(), v.ID()}
	tokens := []string{}
	for i, id := range ids {
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,$2,$3,'fixture',true,$4)", id, []string{"org_admin", "org_manager", "org_staff", "org_outsider", "org_second"}[i], []string{"a@fixture.invalid", "m@fixture.invalid", "s@fixture.invalid", "o@fixture.invalid", "z@fixture.invalid"}[i], i == 0)
		token, _, e := v.Issue(id, v.Now()+3600, 0)
		if e != nil {
			t.Fatal(e)
		}
		tokens = append(tokens, token)
	}
	// Base account team visibility is automatic; organisation administration remains explicit.
	journal := func(c database.Connection) events.Writer { return eventmodule.New().Writer(c) }
	s := app.NewOrganisation(out.NewOrganisationUnitOfWork(pool, journal), v)
	dep, e := s.SaveDepartment(ctx, tokens[0], "", domain.DepartmentInput{Code: "OPS", Name: "Operations", IsActive: true, Reason: "Initial department setup"})
	if e != nil {
		t.Fatal(e)
	}
	input := func(manager string, root bool) domain.EmployeeInput {
		return domain.EmployeeInput{FirstName: "Fixture", LastName: "Employee", Mobile: "+91 9000000000", DepartmentID: dep.ID, ManagerID: manager, IsCompanyRoot: root, Reason: "Initial reporting line setup"}
	}
	if _, e = s.SaveEmployee(ctx, tokens[0], ids[0], input("", true)); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveEmployee(ctx, tokens[0], ids[1], input(ids[0], false)); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveEmployee(ctx, tokens[0], ids[2], input(ids[1], false)); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveEmployee(ctx, tokens[0], ids[3], input(ids[0], false)); e != nil {
		t.Fatal(e)
	}
	team, e := s.Team(ctx, tokens[1])
	if e != nil || len(team) != 1 || team[0].ID != ids[2] {
		t.Fatal("manager ancestry scope", len(team), e)
	}
	if _, e = s.Departments(ctx, tokens[1]); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("manager gained organisation admin", e)
	}
	if _, e = s.SaveEmployee(ctx, tokens[0], ids[2], input(ids[1], false)); !errors.Is(e, domain.ErrOrganisationConflict) {
		t.Fatal("stale profile accepted", e)
	}
	cycle := input(ids[2], false)
	cycle.Version = 1
	if _, e = s.SaveEmployee(ctx, tokens[0], ids[0], cycle); e == nil {
		t.Fatal("reporting cycle accepted")
	}
	if _, e = s.SaveDepartment(ctx, tokens[0], dep.ID, domain.DepartmentInput{Code: "OPS", Name: "Operations", Version: 1, Reason: "Attempt inactive department"}); e == nil {
		t.Fatal("department with active employees deactivated")
	}
	if _, e = pool.Exec(ctx, "UPDATE auth.auth_organisation_changes SET reason='rewrite audit evidence'"); e == nil {
		t.Fatal("immutable audit changed")
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=false WHERE id=$1", ids[3])
	inactive := input(ids[3], false)
	inactive.Version = 1
	if _, e = s.SaveEmployee(ctx, tokens[0], ids[2], inactive); e == nil {
		t.Fatal("inactive reporting manager accepted")
	}
	execFixture(t, pool, "UPDATE auth.users SET is_active=true WHERE id=$1", ids[3])
	// An audit writer failure must roll back both the reporting line and contact details.
	broken := app.NewOrganisation(out.NewOrganisationUnitOfWork(pool, nil), v)
	change := input(ids[1], false)
	change.Version = 1
	change.FirstName = "Changed"
	if _, e = broken.SaveEmployee(ctx, tokens[0], ids[2], change); e == nil {
		t.Fatal("missing audit writer accepted")
	}
	var first string
	var version int64
	if e = pool.QueryRow(ctx, "SELECT u.first_name,r.version FROM auth.users u JOIN auth.auth_reporting_lines r ON r.user_id=u.id WHERE u.id=$1", ids[2]).Scan(&first, &version); e != nil || first != "Fixture" || version != 1 {
		t.Fatal("partial profile commit", first, version, e)
	}
	// Opposite concurrent edges cannot both commit. Use two roots to start.
	if _, e = s.SaveEmployee(ctx, tokens[0], ids[4], input("", true)); e != nil {
		t.Fatal(e)
	}
	a, b := input(ids[4], false), input(ids[0], false)
	a.Version = 1
	b.Version = 1
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, work := range []func() error{func() error { _, e := s.SaveEmployee(ctx, tokens[0], ids[0], a); return e }, func() error { _, e := s.SaveEmployee(ctx, tokens[0], ids[4], b); return e }} {
		wg.Add(1)
		go func() { defer wg.Done(); results <- work() }()
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("opposite edges were not serialized", success)
	}
}
