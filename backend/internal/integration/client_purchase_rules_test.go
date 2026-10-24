package integration

import (
	"bytes"
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	eventout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"net/http/httptest"
	"testing"
)

func TestClientPurchasePoliciesRevisionScopeAuditAndSourceGate(t *testing.T) {
	f := newPurchaseFixture(t)
	ctx := context.Background()
	ids := projectIdentities{"writer": {ID: f.actor, Active: true, Permissions: []string{"settings.purchase_rules.read", "settings.purchase_rules.write"}}, "reader": {ID: f.manager, Active: true, Permissions: []string{"settings.purchase_rules.read"}}}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventout.NewJournal(c)) }
	build := func(j func(database.Connection) events.Writer) *purchaseApp.ClientRulesService {
		return purchaseApp.NewClientRules(purchaseRepository.NewClientRulesUnitOfWork(f.pool, f.providers, j), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{})
	}
	service := build(journal)
	data := purchaseDomain.ClientRulesData{ProjectID: nil, ScopeCode: "CAB", Mode: "approved_only", Allowed: []string{f.make}, Excluded: []string{}, EffectiveFrom: "2026-10-01", Evidence: "Reviewed client make schedule"}
	input := purchaseDomain.ClientRulesSave{Data: data, Version: 1, Active: true, Reason: "Record client purchasing instruction"}
	if _, e := service.Save(ctx, "reader", f.client, "", input); !errors.Is(e, purchaseDomain.ClientRulesForbidden) {
		t.Fatal("reader mutated rules", e)
	}
	if _, e := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Save(ctx, "writer", f.client, "", input); e == nil {
		t.Fatal("event failure committed")
	}
	var count int
	if e := f.pool.QueryRow(ctx, "SELECT count(*) FROM client_purchase_rules").Scan(&count); e != nil || count != 0 {
		t.Fatal("rule rollback", e)
	}
	record, e := service.Save(ctx, "writer", f.client, "", input)
	if e != nil || record.Version != 1 || record.CreatedBy == nil || *record.CreatedBy != f.actor {
		t.Fatal("creation", e)
	}
	eval := func(client, project string) (purchaseDomain.ClientRulesVerdict, error) {
		var verdict purchaseDomain.ClientRulesVerdict
		e := database.NewManager(f.pool).Within(ctx, func(ctx context.Context, c database.Connection) error {
			var e error
			verdict, e = purchaseRepository.NewClientRulesEvaluator(c).Evaluate(ctx, purchaseDomain.ClientRulesEvaluation{ClientID: client, ProjectID: project, Category: "CAB", Family: "LTPC", Configuration: "LTPC", Manufacturer: f.make, Today: "2026-10-10"})
			return e
		})
		return verdict, e
	}
	if v, e := eval(f.client, f.project); e != nil || v.Status != "eligible" || v.Rules[0].ID != record.ID {
		t.Fatal("native policy source", v, e)
	}
	if v, e := eval(f.otherClient, f.otherProject); e != nil || v.Status != "eligible" || len(v.Rules) != 0 || len(v.Reasons) != 0 {
		t.Fatal("policy crossed clients", v, e)
	}
	project := f.otherProject
	bad := input
	bad.Data.ProjectID = &project
	if _, e := service.Save(ctx, "writer", f.client, "", bad); e == nil {
		t.Fatal("foreign-client project policy")
	}
	exclusion := input
	project = f.project
	exclusion.Data = purchaseDomain.ClientRulesData{ProjectID: &project, ScopeCode: "LTPC", Mode: "unrestricted", Allowed: []string{}, Excluded: []string{f.make}, EffectiveFrom: "2026-10-01", Evidence: "Client exclusion supersedes general make list"}
	if _, e := service.Save(ctx, "writer", f.client, "", exclusion); e != nil {
		t.Fatal(e)
	}
	if v, e := eval(f.client, f.project); e != nil || v.Status != "blocked" || len(v.Rules) != 2 {
		t.Fatal("project relaxed exclusion", v, e)
	}
	input.Version = record.Version
	input.Data.Evidence = "Client issued revised approved schedule"
	updated, e := service.Save(ctx, "writer", f.client, record.ID, input)
	if e != nil || updated.Version != 2 || updated.UpdatedBy == nil || *updated.UpdatedBy != f.actor {
		t.Fatal("revision", e)
	}
	if _, e := service.Save(ctx, "writer", f.client, record.ID, input); !errors.Is(e, purchaseDomain.ClientRulesStale) {
		t.Fatal("stale overwrite", e)
	}
	history, e := service.History(ctx, "reader", f.client, record.ID)
	if e != nil || len(history) != 2 || history[0].Before == nil || history[0].Before.Version != 1 || history[0].After.Version != 2 {
		t.Fatal("before/after history", e)
	}
	if _, e := service.History(ctx, "reader", f.otherClient, record.ID); !errors.Is(e, purchaseDomain.ClientRulesMissing) {
		t.Fatal("history crossed clients", e)
	}
	for _, sql := range []string{"UPDATE client_purchase_rule_events SET reason='rewrite'", "DELETE FROM client_purchase_rule_events", "TRUNCATE client_purchase_rule_events", "DELETE FROM client_purchase_rules", "TRUNCATE client_purchase_rules CASCADE"} {
		if _, e := f.pool.Exec(ctx, sql); e == nil {
			t.Fatal("policy evidence mutable", sql)
		}
	}
	router := chi.NewRouter()
	purchaseHTTP.ClientRulesRegister(router, service)
	for _, body := range []string{`null`, `{"unexpected":true}`, `{"data":null}`, `{"data":{"allowed":null}}`, `{"data":{},"is_active":null}`, `{"data":{},"reason":null}`, `{} {}`} {
		request := httptest.NewRequest("POST", "/api/v1/clients/"+f.client+"/purchase-rules", bytes.NewBufferString(body))
		request.Header.Set("Authorization", "Bearer writer")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != 422 {
			t.Fatal("strict policy request", response.Code, body)
		}
	}
	execFixture(t, f.pool, "UPDATE manufacturers SET is_active=false WHERE id=$1", f.make)
	input.Version = updated.Version
	if _, e := service.Save(ctx, "writer", f.client, record.ID, input); e == nil {
		t.Fatal("inactive manufacturer policy")
	}
	m, e := schema.Open(ctx, f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Migrate(27); e == nil {
		t.Fatal("populated policy rollback removed history")
	}
}
