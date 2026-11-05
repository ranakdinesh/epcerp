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
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	out "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
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
	if e != nil || !strings.HasPrefix(u.Path, "/epcerp_auth_qa_team_") {
		panic("isolated Project QA database required")
	}
	var v struct {
		Actor, Method, Path, Operation, Project, Code string
		Write                                         bool
		Body                                          json.RawMessage
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
	if v.Operation == "gate" {
		identity, err := (resolver{p, v.Actor}).Resolve(ctx, "")
		if err != nil {
			panic(err)
		}
		var result domain.Access
		err = database.NewManager(p).Within(ctx, func(ctx context.Context, c database.Connection) error {
			var e error
			result, e = out.NewGate(c, access.Providers(c)).Check(ctx, domain.Actor{ID: identity.ID, Active: identity.Active, Superuser: identity.Superuser, Permissions: identity.Permissions}, v.Project, v.Code, v.Write)
			return e
		})
		status := 200
		if e, ok := err.(*domain.Error); ok {
			status = e.Status
		} else if err != nil {
			panic(err)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": status, "body": result})
		return
	}
	r := chi.NewRouter()
	_ = os.Setenv("EPCERP_PROJECT_TEAM_OWNER", "go")
	if e = access.New(r, p, resolver{p, v.Actor}); e != nil {
		panic(e)
	}
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
