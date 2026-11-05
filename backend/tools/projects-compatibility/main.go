// HTTP parity oracle restricted to explicitly isolated Project QA databases.
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
	ec "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	mr "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	ma "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	mc "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects"
	in "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/projects/application"
	pr "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	pc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	calendar "github.com/ranakdinesh/epcERP/internal/modules/work_calendar"
	cc "github.com/ranakdinesh/epcERP/internal/modules/work_calendar/contracts"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
)

type resolver struct {
	p  *pgxpool.Pool
	id string
}

func (a resolver) Resolve(ctx context.Context, _ string) (auth.Identity, error) {
	u, e := aa.NewAssignmentDirectory(ar.NewAssignmentReader(a.p)).Get(ctx, a.id)
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
	if e != nil || !strings.HasPrefix(u.Path, "/epcerp_auth_qa_projects_") {
		panic("isolated Project QA database required")
	}
	var v struct {
		Actor, Method, Path, Operation string
		Body                           json.RawMessage
	}
	if e = json.NewDecoder(os.Stdin).Decode(&v); e != nil {
		panic(e)
	}
	ctx := context.Background()
	p, e := pgxpool.New(ctx, u.String())
	if e != nil {
		panic(e)
	}
	defer p.Close()
	users := func(c database.Connection) auth.AssignmentDirectory {
		return aa.NewAssignmentDirectory(ar.NewAssignmentReader(c))
	}
	journal := func(c database.Connection) ec.Writer { return ea.NewRecorder(er.NewJournal(c)) }
	unit := out.NewUnitOfWork(p, func(c database.Connection) mc.ClientDirectory {
		return ma.NewClientDirectory(mr.NewClientIdentityReader(c))
	}, users, journal, func(c database.Connection) cc.Reader { return calendar.Reader(c) }).WithPurchasing(func(c database.Connection) pc.WorkspaceOwnership {
		return pr.NewWorkspaceOwnership(c, users(c), journal(c))
	}).WithReadableAssignments(access.ReadableProjects)
	if v.Operation == "recalculate" {
		var count int
		err := database.NewManager(p).Within(ctx, func(ctx context.Context, c database.Connection) error {
			var err error
			count, err = projects.CalendarUpdater(c, journal(c), calendar.Reader(c))(ctx, v.Actor)
			return err
		})
		result := map[string]any{"count": count}
		if err != nil {
			result["error"] = err.Error()
		}
		if e = json.NewEncoder(os.Stdout).Encode(result); e != nil {
			panic(e)
		}
		return
	}
	svc := app.New(unit, out.NewAuthBridge(resolver{p, v.Actor}), out.SystemValues{})
	r := chi.NewRouter()
	in.Register(r, svc)
	in.RegisterMetadata(r, svc.Metadata())
	req := httptest.NewRequest(v.Method, "/api/v1/projects"+v.Path, strings.NewReader(string(v.Body)))
	req.Header.Set("Authorization", "Bearer synthetic")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body any
	if e = json.Unmarshal(w.Body.Bytes(), &body); e != nil {
		panic(e)
	}
	if e = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": w.Code, "body": body}); e != nil {
		panic(e)
	}
}
