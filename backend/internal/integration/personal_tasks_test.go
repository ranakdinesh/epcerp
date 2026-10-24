package integration

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	in "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/application"
	d "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/domain"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseContracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	"net/http/httptest"
	"testing"
)

func verifyPersonalPurchaseTasks(t *testing.T, f purchaseFixture, ids projectIdentities, p purchaseRepository.WorkflowFactory, pkg, state string, blocked, execute bool) {
	t.Helper()
	ctx := t.Context()
	s := app.New(out.NewUnitOfWork(f.pool, func(c database.Connection) out.Providers {
		return out.Providers{Users: authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c)), Purchase: purchaseRepository.NewWorkflowTaskReader(c, p(c))}
	}), out.NewAuthBridge(ids), out.Values{})

	if state == "not_started" {
		manager := database.NewManager(f.pool)
		if e := manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
			providers := p(c)
			record, _, e := purchaseRepository.NewWorkflowSourceReader(c, providers).Inspect(ctx, f.project, pkg)
			if e != nil {
				return e
			}
			feed := purchaseRepository.NewWorkflowProgressReader(c, providers)
			page, e := feed.Page(ctx, f.project, []string{record.DemandID}, 1, 20)
			if e != nil {
				return e
			}
			if page.Total != 1 || len(page.Rows) != 1 || page.Rows[0].Package.ID != pkg || page.Rows[0].Source.DemandID != record.DemandID {
				t.Fatal("owned progress source page", page)
			}
			page, e = feed.Page(ctx, f.project, []string{record.DemandID}, 2, 20)
			if e != nil {
				return e
			}
			if page.Total != 1 || len(page.Rows) != 0 {
				t.Fatal("bounded progress pagination", page)
			}
			page, e = feed.Page(ctx, f.otherProject, []string{record.DemandID}, 1, 20)
			if e != nil {
				return e
			}
			if page.Total != 0 {
				t.Fatal("cross-project progress package leak")
			}
			if _, e = feed.Page(ctx, f.project, []string{record.DemandID, record.DemandID}, 1, 20); e == nil {
				t.Fatal("duplicate demand scope")
			}
			return nil
		}); e != nil {
			t.Fatal("native progress source", e)
		}
		if _, e := purchaseRepository.NewWorkflowProgressReader(f.pool, p(f.pool)).Page(ctx, f.project, []string{}, 1, 20); e == nil {
			t.Fatal("unbound progress projection")
		}

		execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=(SELECT id FROM auth.roles WHERE name='package native fixture') AND permission_id=(SELECT id FROM auth.permissions WHERE code='work.tasks.read')")
		if _, e := s.Page(ctx, "package-worker", d.Query{}); e != d.Forbidden {
			t.Fatal("revoked personal grant", e)
		}
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id) SELECT r.id,p.id FROM auth.roles r CROSS JOIN auth.permissions p WHERE r.name='package native fixture' AND p.code='work.tasks.read'")
		execFixture(t, f.pool, "UPDATE auth.users SET is_active=false WHERE id=$1", f.outsider)
		if _, e := s.Page(ctx, "package-worker", d.Query{}); e != d.Unauthorized {
			t.Fatal("inactive personal assignee", e)
		}
		execFixture(t, f.pool, "UPDATE auth.users SET is_active=true WHERE id=$1", f.outsider)
	}
	rows, e := s.Page(ctx, "package-worker", d.Query{})
	want := int64(0)
	if execute {
		want = 1
	}
	if e != nil || rows.Total != want || int64(len(rows.Rows)) != want {
		t.Fatal("outside-project personal assignment", rows, e)
	}
	if execute && (rows.Rows[0].PackageID != pkg || rows.Rows[0].ProjectID != f.project || rows.Rows[0].Number != 5 || rows.Rows[0].State != state || rows.Rows[0].Relation != "doer") {
		t.Fatal("personal projection", rows)
	}
	other, e := s.Page(ctx, "package-manager", d.Query{Role: "resolver"})
	want = 0
	if blocked {
		want = 1
	}
	if e != nil || other.Total != want {
		t.Fatal("only active resolver issue queued", other, e)
	}
	empty, e := s.Page(ctx, "package-worker", d.Query{Search: "missing source % _"})
	if e != nil || empty.Total != 0 || len(empty.Rows) != 0 {
		t.Fatal("search boundaries", empty, e)
	}
	empty, e = s.Page(ctx, "package-worker", d.Query{Page: 2})
	if e != nil || len(empty.Rows) != 0 {
		t.Fatal("server pagination", empty, e)
	}
	router := chi.NewRouter()
	in.Register(router, s)
	for _, test := range []struct {
		token, path string
		code        int
	}{{"package-worker", "/api/v1/my-tasks", 200}, {"", "/api/v1/my-tasks", 401}, {"package-worker", "/api/v1/my-tasks?user_id=other", 422}, {"package-worker", "/api/v1/my-tasks?role=resolver", 200}} {
		req := httptest.NewRequest("GET", test.path, nil)
		req.Header.Set("Authorization", "Bearer "+test.token)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != test.code {
			t.Fatal("personal HTTP", test.code, res.Code, res.Body.String())
		}
	}
	if _, e := purchaseRepository.NewWorkflowTaskReader(f.pool, p(f.pool)).Assigned(ctx, purchaseContracts.WorkflowActor{ID: f.outsider, Active: true}, purchaseContracts.WorkflowTaskQuery{Page: 1, Size: 20, Role: "all", State: "all"}); e == nil {
		t.Fatal("unbound task reader")
	}
}
