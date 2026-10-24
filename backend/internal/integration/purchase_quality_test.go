package integration

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	purchasePorts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func verifyNativePurchaseQuality(t *testing.T, f purchaseFixture, ids projectIdentities, wp purchaseRepository.WorkflowFactory, ws *purchaseApp.WorkflowService, pkg string, rp func(database.Connection) purchaseRepository.InspectionReportsProviders, reports *purchaseApp.InspectionReportsService) {
	t.Helper()
	ctx := t.Context()
	v := purchaseRepository.Values{}
	role := v.ID()
	codes := []string{"projects.purchase_quality.read", "projects.purchase_quality.write", "projects.purchase_quality.verify"}
	execFixture(t, f.pool, "INSERT INTO auth.roles(id,name,description,is_active)VALUES($1,$2,'native quality',true)", role, "Native quality "+role)
	for _, code := range codes {
		execFixture(t, f.pool, "INSERT INTO auth.permissions(id,code,description)VALUES($1,$2,'native quality')ON CONFLICT(code)DO NOTHING", v.ID(), code)
		execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code=$2", role, code)
	}
	execFixture(t, f.pool, "INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)", f.actor, role)
	actor := ids["report-writer"]
	actor.Permissions = append(append([]string{}, actor.Permissions...), codes...)
	ids["quality-writer"] = actor
	w, e := ws.Workspace(ctx, "loi-manager", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ws.Assign(ctx, "loi-manager", f.project, pkg, purchaseDomain.WorkflowAssign{Version: w.Package.Version, Reason: "Assign actual quality doer", Rows: []purchaseDomain.WorkflowAssignment{{Number: 14, Team: "Procurement", DoerID: f.actor, ResolverID: f.manager, PlannedDate: purchaseDomain.QualityToday(v.Now())}}}); e != nil {
		t.Fatal(e)
	}
	providers := func(c database.Connection) purchaseRepository.QualityProviders {
		x := rp(c)
		return purchaseRepository.QualityProviders{Projects: x.Projects, Purchase: x.Purchase, Access: x.Access, Users: x.Users, Work: x.Work, Reports: purchaseRepository.NewInspectionReportsIssuedReader(c, x)}
	}
	factory := func(c database.Connection) purchasePorts.QualityContext {
		return purchaseRepository.NewQualityContext(providers(c))
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.QualityService {
		return purchaseApp.NewQuality(purchaseRepository.NewQualityUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), v)
	}
	service := build(purchaseJournal)
	failed := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} })
	api := chi.NewRouter()
	purchaseHTTP.QualityRegister(api, service)
	base := "/api/v1/projects/" + f.project + "/purchase-workflow/" + pkg + "/quality"
	request := func(method, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, base, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r := httptest.NewRecorder()
		api.ServeHTTP(r, req)
		return r
	}
	initial, e := service.Workspace(ctx, "quality-writer", f.project, pkg)
	if e != nil || initial.Record != nil || initial.Source.ReportVersion != 6 || initial.Source.Quantity != "1.000001" || initial.Eligibility.Ready || initial.Eligibility.RequiredCount != 5 {
		t.Fatal("actual report source", e, initial.Source)
	}
	if request("GET", "", "").Code != 401 || request("POST", "quality-writer", `{"version":0,"action":"record","reason":"r","certificate_code":"TC","details":{"verified":true}}`).Code != 422 {
		t.Fatal("HTTP quality protection")
	}
	q := purchaseDomain.QualityCommand{Action: "record", Code: "TC", Reason: "Actual type test evidence", Details: &purchaseDomain.QualityDetails{Number: "TC-1", Issuer: "Actual test lab", DocumentReference: "actual-certificate.pdf"}}
	if leaked, e := failed.Execute(ctx, "quality-writer", f.project, pkg, q); e == nil || leaked.Record != nil {
		t.Fatal("late journal failure leaked quality")
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_quality_cases").Scan(&count); e != nil || count != 0 {
		t.Fatal("failed case persisted", e)
	}
	raw, _ := json.Marshal(q)
	response := request("POST", "quality-writer", string(raw))
	var current purchaseDomain.QualityWorkspace
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &current) != nil || current.Record == nil || len(current.Record.Certificates) != 5 || len(current.History) != 1 {
		t.Fatal("quality HTTP", response.Code, response.Body.String())
	}
	checkProgress := func() {
		t.Helper()
		want := "not_started"
		if current.Eligibility.Ready {
			want = "completed"
		} else if current.Eligibility.VerifiedCount > 0 {
			want = "in_progress"
		}
		if err := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			p, err := purchaseRepository.NewQualityProgressReader(c).Progress(txctx, f.project, pkg, current.Source.QualitySourceFacts, true)
			if err != nil || p.State != want || !p.Current || p.VerifiedCount != current.Eligibility.VerifiedCount {
				t.Fatal("actual quality progress", err, p)
			}
			if want == "completed" && (p.Snapshot == nil || len(p.Snapshot.Certificates) != 5) {
				t.Fatal("quality completed without certificate evidence", p)
			}
			changed := current.Source.QualitySourceFacts
			changed.ReportRevision++
			stale, err := purchaseRepository.NewQualityProgressReader(c).Progress(txctx, f.project, pkg, changed, true)
			if err != nil || stale.State != "blocked" || stale.Current {
				t.Fatal("obsolete IRN earned quality completion", err, stale)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := purchaseRepository.NewQualityProgressReader(f.pool).Progress(ctx, f.project, pkg, current.Source.QualitySourceFacts, true); err == nil {
			t.Fatal("unbound quality progress reader accepted pool")
		}
	}
	checkProgress()
	id := current.Record.ID
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_quality_certificates WHERE case_id=$1", id).Scan(&count); e != nil || count != 5 {
		t.Fatal("owned children", e, count)
	}
	verify := purchaseDomain.QualityCommand{Version: 1, Action: "verify", Code: "TC", Reason: "Reviewed actual certificate"}
	execFixture(t, f.pool, "DELETE FROM auth.role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM auth.permissions WHERE code='projects.purchase_quality.verify')", role)
	if _, e = service.Execute(ctx, "quality-writer", f.project, pkg, verify); e == nil {
		t.Fatal("edit granted verification")
	}
	execFixture(t, f.pool, "INSERT INTO auth.role_permissions(role_id,permission_id)SELECT $1,id FROM auth.permissions WHERE code='projects.purchase_quality.verify'", role)
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=false WHERE id=$1", f.project)
	held, e := service.Workspace(ctx, "quality-writer", f.project, pkg)
	if e != nil || held.Record == nil || len(held.CertificateActions["TC"]) > 0 {
		t.Fatal("held case", e)
	}
	if _, e = service.Execute(ctx, "quality-writer", f.project, pkg, verify); e == nil {
		t.Fatal("held verification accepted")
	}
	execFixture(t, f.pool, "UPDATE projects SET purchase_enabled=true WHERE id=$1", f.project)
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, e := service.Execute(ctx, "quality-writer", f.project, pkg, verify)
			results <- e
		}()
	}
	group.Wait()
	close(results)
	wins := 0
	for e := range results {
		if e == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal("concurrent verification winners", wins)
	}
	current, e = service.Workspace(ctx, "quality-writer", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	execute := func(q purchaseDomain.QualityCommand) {
		t.Helper()
		q.Version = current.Record.Version
		var err error
		current, err = service.Execute(ctx, "quality-writer", f.project, pkg, q)
		if err != nil {
			t.Fatal(q.Action, q.Code, err)
		}
		checkProgress()
	}
	for _, code := range []string{"TR", "MTC", "RTC", "Warranty"} {
		execute(purchaseDomain.QualityCommand{Action: "record", Code: code, Reason: "Actual certificate details", Details: &purchaseDomain.QualityDetails{Number: code + "-1", Issuer: "Actual laboratory"}})
		execute(purchaseDomain.QualityCommand{Action: "verify", Code: code, Reason: "Review actual certificate"})
	}
	if !current.Eligibility.Ready || current.Eligibility.VerifiedCount != 5 {
		t.Fatal("all required readiness", current.Eligibility)
	}
	if _, e = purchaseRepository.NewQualityVerifiedReader(f.pool, providers(f.pool)).Verified(ctx, f.project, pkg); e == nil {
		t.Fatal("unbound quality reader accepted")
	}
	fact := func(want bool) {
		t.Helper()
		if e := database.NewManager(f.pool).Within(ctx, func(txctx context.Context, c database.Connection) error {
			r, e := purchaseRepository.NewQualityVerifiedReader(c, providers(c)).Verified(txctx, f.project, pkg)
			if want && (e != nil || r == nil || r.ID != id) || !want && e == nil {
				t.Fatal("current quality fact", want, e)
			}
			return nil
		}); e != nil {
			t.Fatal(e)
		}
	}
	fact(true)
	execute(purchaseDomain.QualityCommand{Action: "record", Code: "TC", Reason: "Correct actual certificate", Details: &purchaseDomain.QualityDetails{Number: "TC-2", Issuer: "Actual test lab"}})
	if current.Record.Certificates[0].Verified || current.Eligibility.Ready {
		t.Fatal("metadata edit retained attestation")
	}
	fact(false)
	execute(purchaseDomain.QualityCommand{Action: "verify", Code: "TC", Reason: "Reverify corrected actual document"})
	fact(true)
	execute(purchaseDomain.QualityCommand{Action: "revoke", Code: "TC", Reason: "Withdraw reviewed attestation"})
	fact(false)
	execute(purchaseDomain.QualityCommand{Action: "verify", Code: "TC", Reason: "Re-review actual document"})
	fact(true)
	execute(purchaseDomain.QualityCommand{Action: "record", Code: "TC", Reason: "Record expired evidence truthfully", Details: &purchaseDomain.QualityDetails{Number: "TC-old", Issuer: "Actual lab", ValidUntil: "2000-01-01"}})
	if _, e = service.Execute(ctx, "quality-writer", f.project, pkg, purchaseDomain.QualityCommand{Version: current.Record.Version, Action: "verify", Code: "TC", Reason: "Expired"}); e == nil {
		t.Fatal("expired certificate verified")
	}
	fact(false)
	execute(purchaseDomain.QualityCommand{Action: "record", Code: "TC", Reason: "Record current replacement evidence", Details: &purchaseDomain.QualityDetails{Number: "TC-new", Issuer: "Actual lab"}})
	execute(purchaseDomain.QualityCommand{Action: "verify", Code: "TC", Reason: "Verify current replacement evidence"})
	fact(true)
	prior, e := reports.Workspace(ctx, "report-writer", f.project, pkg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = reports.Execute(ctx, "report-writer", f.project, pkg, purchaseDomain.InspectionReportsCommand{Version: prior.Record.Version, Action: "revise", Reason: "Actual upstream report correction"}); e != nil {
		t.Fatal(e)
	}
	fact(false)
	r := prior.Record.Report
	if _, e = reports.Execute(ctx, "report-writer", f.project, pkg, purchaseDomain.InspectionReportsCommand{Version: prior.Record.Version + 1, Action: "record", Reason: "Issue corrected actual IRN", Report: &purchaseDomain.InspectionReportsReportInput{Status: r.Status, IRNumber: r.IRNumber, IRNNumber: r.IRNNumber, IssueDate: r.IssueDate, Authority: r.Authority, Quantity: r.Quantity, DocumentReference: r.DocumentReference}}); e != nil {
		t.Fatal(e)
	}
	if _, e = service.Execute(ctx, "quality-writer", f.project, pkg, purchaseDomain.QualityCommand{Version: current.Record.Version, Action: "verify", Code: "TC", Reason: "Old source"}); e == nil {
		t.Fatal("obsolete attestation accepted")
	}
	execute(purchaseDomain.QualityCommand{Action: "revise", Reason: "Review against corrected actual IRN"})
	if current.Eligibility.VerifiedCount != 0 || current.Record.Certificates[0].Number != "TC-new" {
		t.Fatal("revision reset or metadata lost")
	}
	for _, code := range []string{"TC", "TR", "MTC", "RTC", "Warranty"} {
		execute(purchaseDomain.QualityCommand{Action: "verify", Code: code, Reason: "Review certificate against current IRN"})
	}
	fact(true)
	beforeVersion := current.Record.Version
	if leaked, err := failed.Execute(ctx, "quality-writer", f.project, pkg, purchaseDomain.QualityCommand{Version: beforeVersion, Action: "revoke", Code: "TC", Reason: "Must roll back late journal failure"}); err == nil || leaked.Record != nil {
		t.Fatal("failed correction escaped")
	}
	unchanged, err := service.Workspace(ctx, "quality-writer", f.project, pkg)
	if err != nil || unchanged.Record.Version != beforeVersion || !unchanged.Eligibility.Ready {
		t.Fatal("failed verification correction changed prior truth", err)
	}
	// Match every child projection but omit the audited parent change: the deferred
	// child/case equality guard, rather than a scalar CHECK, must reject the commit.
	if _, err = f.pool.Exec(ctx, `UPDATE purchase_quality_certificates SET verified=false,verified_at=NULL,verified_by_id=NULL,data=data||jsonb_build_object('verified',false,'verified_at',NULL,'verified_by_id','') WHERE case_id=$1 AND code='TC'`, id); err == nil {
		t.Fatal("unaudited child attestation bypassed deferred parent audit")
	}
	fact(true)
	for _, sql := range []string{"UPDATE purchase_quality_cases SET version=version+1 WHERE id=$1", "DELETE FROM purchase_quality_cases WHERE id=$1", "DELETE FROM purchase_quality_certificates WHERE case_id=$1", "DELETE FROM purchase_quality_events WHERE subject_id=$1", "UPDATE purchase_quality_certificates SET cert_number='forged' WHERE case_id=$1"} {
		if _, e = f.pool.Exec(ctx, sql, id); e == nil {
			t.Fatal("unaudited evidence mutation allowed")
		}
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='purchase_quality' AND entity_id=$1", id).Scan(&count); e != nil || count != 23 || len(current.History) != 23 {
		t.Fatal("atomic quality history", e, count, len(current.History))
	}
	verifyNativeClientClearance(t, f, ids, ws, pkg, providers, service)
}
