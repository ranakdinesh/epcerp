// HTTP parity oracle restricted to explicitly isolated Project QA databases.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	eventsout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventsapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	accounts "github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance"
	tasks "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks"
	access "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	gateout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	gatedomain "github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
	budgets "github.com/ranakdinesh/epcERP/internal/modules/project_budgets"
	budgetdomain "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	items "github.com/ranakdinesh/epcERP/internal/modules/project_items"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"mime"
	"mime/multipart"
	"net/textproto"

	ar "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	aa "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	composition "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/composition"

	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"time"
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
	if e != nil || !strings.HasPrefix(u.Path, "/epcerp_auth_qa_purchase_") {
		panic("isolated Project QA database required")
	}
	var v struct {
		Actor, Method, Path, Operation, Project, Code, Name, Mime, Data string
		Binary                                                          bool
		Write                                                           bool
		Body                                                            json.RawMessage
		Fields                                                          map[string]string
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
		var result gatedomain.Access
		err = database.NewManager(p).Within(ctx, func(ctx context.Context, c database.Connection) error {
			var e error
			result, e = gateout.NewGate(c, access.Providers(c)).Check(ctx, gatedomain.Actor{ID: identity.ID, Active: identity.Active, Superuser: identity.Superuser, Permissions: identity.Permissions}, v.Project, v.Code, v.Write)
			return e
		})
		status := 200
		if e, ok := err.(*gatedomain.Error); ok {
			status = e.Status
		} else if err != nil {
			panic(err)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": status, "can_write": result.CanWrite})
		return
	}
	if v.Operation == "budget_fingerprint" {
		var d budgetdomain.Data
		if e = json.Unmarshal(v.Body, &d); e != nil {
			panic(e)
		}
		data, err := json.Marshal(d)
		if err != nil {
			panic(err)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"fingerprint": fmt.Sprintf("%x", sha256.Sum256(data))})
		return
	}
	if v.Operation == "approval_fingerprint" {
		var a domain.QuotationApprovalsAttempt
		if e = json.Unmarshal(v.Body, &a); e != nil {
			panic(e)
		}
		data, err := json.Marshal(a)
		if err != nil {
			panic(err)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"fingerprint": fmt.Sprintf("%x", sha256.Sum256(data))})
		return
	}
	if v.Operation == "progress_build" {
		var input struct {
			Source domain.ProgressSource
			At     time.Time
		}
		if e = json.Unmarshal(v.Body, &input); e != nil {
			panic(e)
		}
		result, err := domain.ProgressBuild(input.Source, input.At)
		if err != nil {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"error": err.Error()})
		} else {
			_ = json.NewEncoder(os.Stdout).Encode(result)
		}
		return
	}

	r := chi.NewRouter()
	budgets.New(r, p, resolver{p, v.Actor})
	purchase, e := composition.New(r, p, resolver{p, v.Actor})
	if e != nil {
		panic(e)
	}
	items.New(items.Dependencies{Router: r, Database: p, Identity: resolver{p, v.Actor}, Providers: purchase.ProjectItems, Events: func(c database.Connection) events.Writer { return eventsapp.NewRecorder(eventsout.NewJournal(c)) }})
	accounts.New(accounts.Dependencies{Router: r, Database: p, Identity: resolver{p, v.Actor}, Providers: purchase.Accounts, Events: func(c database.Connection) events.Writer { return eventsapp.NewRecorder(eventsout.NewJournal(c)) }})
	if err := tasks.New(tasks.Dependencies{Router: r, Database: p, Identity: resolver{p, v.Actor}, Users: purchase.TaskProviders.Users, Quotations: purchase.TaskProviders.Quotations, Workflow: purchase.TaskProviders.Workflow}); err != nil {
		panic(err)
	}

	req := httptest.NewRequest(v.Method, "/api/v1"+v.Path, strings.NewReader(string(v.Body)))
	req.Header.Set("Authorization", "Bearer synthetic")
	req.Header.Set("Content-Type", "application/json")
	if v.Operation == "upload" {
		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		for name, value := range v.Fields {
			if err := writer.WriteField(name, value); err != nil {
				panic(err)
			}
		}
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": v.Name}))
		header.Set("Content-Type", v.Mime)
		part, err := writer.CreatePart(header)
		if err != nil {
			panic(err)
		}
		data, err := base64.StdEncoding.DecodeString(v.Data)
		if err != nil {
			panic(err)
		}
		if _, err = part.Write(data); err != nil {
			panic(err)
		}
		if err = writer.Close(); err != nil {
			panic(err)
		}
		req = httptest.NewRequest("POST", "/api/v1"+v.Path, &buffer)
		req.Header.Set("Authorization", "Bearer synthetic")
		req.Header.Set("Content-Type", writer.FormDataContentType())
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if v.Binary {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": w.Code, "base64": base64.StdEncoding.EncodeToString(w.Body.Bytes()), "headers": w.Header()})
		return
	}
	var body any
	if e = json.Unmarshal(w.Body.Bytes(), &body); e != nil {
		body = w.Body.String()
	}
	if e = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": w.Code, "body": body}); e != nil {
		panic(e)
	}
}
