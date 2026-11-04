// Synthetic, transaction-bound Personal Tasks parity oracle.
package main

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	ar "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	aa "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	er "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	ea "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	mr "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	ma "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	in "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/application"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	ao "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	mo "github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/outbound"
	po "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	pr "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
)

type resolver struct {
	pool *pgxpool.Pool
	id   string
}

func (a resolver) Resolve(ctx context.Context, _ string) (auth.Identity, error) {
	u, e := aa.NewAssignmentDirectory(ar.NewAssignmentReader(a.pool)).Get(ctx, a.id)
	if e != nil {
		return auth.Identity{}, e
	}
	if u == nil {
		return auth.Identity{}, nil
	}
	return auth.Identity{ID: u.ID, Active: u.Active, Superuser: u.Superuser, Permissions: u.Permissions}, nil
}
func main() {
	u, e := url.Parse(os.Getenv("EPCERP_DATABASE_URL"))
	if e != nil || !strings.HasPrefix(u.Path, "/epcerp_auth_qa_tasks_") {
		panic("isolated task QA database required")
	}
	var v struct{ Actor, Query string }
	if e = json.NewDecoder(os.Stdin).Decode(&v); e != nil {
		panic(e)
	}
	ctx := context.Background()
	p, e := pgxpool.New(ctx, u.String())
	if e != nil {
		panic(e)
	}
	defer p.Close()
	unit := out.NewUnitOfWork(p, func(c database.Connection) out.Providers {
		users := aa.NewAssignmentDirectory(ar.NewAssignmentReader(c))
		projects := po.NewContextReader(c)
		clients := ma.NewClientDirectory(mr.NewClientIdentityReader(c))
		scope := pr.NewWorkspaceAccessGate(c, projects, clients)
		items := mr.NewItemReader(c, mr.NewCategoryDirectory(c), mr.NewTemplateItemReader(c), mr.NewManufacturerItemReader(c))
		journal := ea.NewRecorder(er.NewJournal(c))
		gate := ao.NewGate(c, access.Providers(c))
		ip := pr.IndentsProviders{Scope: scope, Items: items, Rules: pr.NewClientRulesEvaluator(c), Holds: pr.NewHoldsEvaluator(c), Projects: projects, Clients: clients, Demands: mo.NewDemandPort(c, mo.Providers{}, journal), Users: users}
		wp := pr.WorkflowProviders{Projects: projects, Purchase: scope, Access: gate, Users: users, Indents: pr.NewIndentsReader(c), Preparation: pr.NewIndentsPreparationReader(c, ip)}
		return out.Providers{Users: users, Purchase: pr.NewWorkflowTaskReader(c, wp), Quotes: pr.NewQuotationWorkReader(c, users, scope, gate, projects), SupplierReviews: mr.NewVendorReviewTasks(c, users, nil, nil)}
	})
	r := chi.NewRouter()
	svc := app.New(unit, out.NewAuthBridge(resolver{p, v.Actor}), out.Values{})
	in.Register(r, svc)
	req := httptest.NewRequest("GET", "/api/v1/my-tasks"+v.Query, nil)
	req.Header.Set("Authorization", "Bearer synthetic")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body any
	if e = json.Unmarshal(w.Body.Bytes(), &body); e != nil {
		panic(e)
	}
	result := map[string]any{"status": w.Code, "body": body}
	if w.Code >= 500 {
		q, err := in.Query(req)
		if err == nil {
			_, err = svc.Page(ctx, "synthetic", q)
		}
		if err != nil {
			result["diagnostic"] = err.Error()
		}
	}
	if e = json.NewEncoder(os.Stdout).Encode(result); e != nil {
		panic(e)
	}
}
