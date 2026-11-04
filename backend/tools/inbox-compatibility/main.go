// HTTP parity oracle, restricted to synthetic Inbox QA databases.
package main

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	ar "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	aa "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	ac "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	in "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/application"
	d "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/domain"
	er "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	ea "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	ec "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
)

type authorizer struct{ id string }

func (a authorizer) Resolve(context.Context, string) (d.Actor, error) {
	return d.Actor{ID: a.id, Active: true, Superuser: true}, nil
}
func main() {
	u, e := url.Parse(os.Getenv("EPCERP_DATABASE_URL"))
	if e != nil || !strings.HasPrefix(u.Path, "/epcerp_auth_qa_inbox_") {
		panic("isolated Inbox QA database required")
	}
	var v struct {
		Actor, Path, Method string
		Body                json.RawMessage
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
	users := func(c database.Connection) ac.AssignmentDirectory {
		return aa.NewAssignmentDirectory(ar.NewAssignmentReader(c))
	}
	journal := func(c database.Connection) ec.Writer { return ea.NewRecorder(er.NewJournal(c)) }
	r := chi.NewRouter()
	in.Register(r, app.New(out.NewUnitOfWork(p, users, journal), authorizer{v.Actor}, out.Values{}))
	req := httptest.NewRequest(v.Method, "/api/v1/inbox"+v.Path, strings.NewReader(string(v.Body)))
	req.Header.Set("Authorization", "Bearer synthetic")
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
