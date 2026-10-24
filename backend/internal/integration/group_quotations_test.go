package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	documentInfra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpmiddleware"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	purchaseHTTP "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	purchaseApp "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
	purchaseDomain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"

	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"

	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

// Called from the complete BO/group/document fixture so evidence is attached to
// genuinely dispatched native documents rather than inserting workflow snapshots.
func verifyNativeQuotationReceipt(t *testing.T, f purchaseFixture, ids projectIdentities, boID, document, lineID, today string, direct bool, allocationMode string) {
	t.Helper()
	ctx := context.Background()
	writer := ids["writer"]
	writer.Permissions = append(writer.Permissions, "projects.group_quotations.view", "projects.group_quotations.read", "projects.group_quotations.write", "projects.group_comparisons.view", "projects.group_comparisons.read")
	ids["quote-writer"] = writer
	outsider := writer
	outsider.ID = f.outsider
	ids["quote-outsider"] = outsider
	comparison := writer
	comparison.Permissions = []string{"projects.purchase_workflow.view", "projects.purchase_workflow.read", "projects.group_comparisons.view", "projects.group_comparisons.read"}
	ids["comparison-reader"] = comparison
	cipher, e := documentInfra.NewDocumentCipher(strings.Repeat("01", 32))
	if e != nil {
		t.Fatal(e)
	}
	factory := func(c database.Connection) purchaseRepository.QuotationsProviders {
		p := f.materialProviders(c, purchaseRepository.NewIndentsReader(c))
		return purchaseRepository.QuotationsProviders{Scope: p.Scope, BO: purchaseRepository.NewBOReader(c), Documents: purchaseRepository.NewIndentDocumentsReader(c, purchaseRepository.NewIndentGroupsReader(c)), Projects: p.Projects, Clients: f.providers(c).Clients, Suppliers: masterRepository.NewVendorDirectory(c)}
	}
	build := func(j func(database.Connection) events.Writer) *purchaseApp.QuotationsService {
		return purchaseApp.NewQuotations(purchaseRepository.NewQuotationsUnitOfWork(f.pool, factory, j), purchaseRepository.NewAuthBridge(ids), purchaseRepository.Values{}, cipher, purchaseRepository.QuotationsInspector{})
	}
	service := build(purchaseJournal)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" && purchaseHTTP.QuotationsIsUploadPath(r.URL.Path) {
				purchaseHTTP.QuotationsUploadGuard(service)(httpmiddleware.PayloadLimit(11<<20)(next)).ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	purchaseHTTP.QuotationsRegister(router, service)
	base := "/api/v1/projects/" + f.project + "/purchase-bo/" + boID + "/group-quotations"
	for _, token := range []string{"", "quote-outsider"} {
		body := &quoteReadProbe{}
		request := httptest.NewRequest("POST", base+"/"+document+"/attachments", body)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		want := 401
		if token != "" {
			want = 403
		}
		if response.Code != want || body.read {
			t.Fatalf("upload body buffered before authorization: %d read=%t", response.Code, body.read)
		}
	}
	for _, raw := range []string{`{"version":null}`, `{"unknown":1}`, `{"version":0,"quote":{"lines":[{"compliant":null}]}}`} {
		request := httptest.NewRequest("POST", base, strings.NewReader(raw))
		request.Header.Set("Authorization", "Bearer quote-writer")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != 422 {
			t.Fatalf("strict HTTP input: %d", response.Code)
		}
	}
	choiceRequest := httptest.NewRequest("GET", "/api/v1/projects/"+f.project+"/quotation-bo?section=comparison", nil)
	choiceRequest.Header.Set("Authorization", "Bearer comparison-reader")
	choiceResponse := httptest.NewRecorder()
	router.ServeHTTP(choiceResponse, choiceRequest)
	if choiceResponse.Code != 200 || !strings.Contains(choiceResponse.Body.String(), boID) || strings.Contains(choiceResponse.Body.String(), "lines") {
		t.Fatal("scoped BO selector without BO permission", choiceResponse.Code)
	}

	data := []byte("%PDF-1.7\nquotation fixture\n%%EOF")
	if _, e = service.Upload(ctx, "quote-outsider", f.project, boID, document, "quote.pdf", "application/pdf", data); !errors.Is(e, purchaseDomain.QuotationsForbidden) {
		t.Fatal("outsider upload", e)
	}
	if _, e = build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Upload(ctx, "quote-writer", f.project, boID, document, "quote.pdf", "application/pdf", data); e == nil {
		t.Fatal("evidence audit failure committed")
	}
	file, e := service.Upload(ctx, "quote-writer", f.project, boID, document, "quote.pdf", "application/pdf", data)
	if e != nil {
		t.Fatal(e)
	}
	var encrypted []byte
	if e = f.pool.QueryRow(ctx, "SELECT ciphertext FROM purchase_quotation_attachments WHERE id=$1", file.ID).Scan(&encrypted); e != nil || bytes.Contains(encrypted, data) {
		t.Fatal("evidence not encrypted", e)
	}
	if _, plain, e := build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Download(ctx, "quote-writer", f.project, boID, document, file.ID); e == nil || len(plain) != 0 {
		t.Fatal("download bytes released before audit commit")
	}
	_, plain, e := service.Download(ctx, "quote-writer", f.project, boID, document, file.ID)
	if e != nil || !bytes.Equal(plain, data) {
		t.Fatal("evidence round trip", e)
	}
	price := "12.35"
	wantTotal := "145.74"
	if direct {
		wantTotal = "145.69"
	}
	months := int32(24)
	command := purchaseDomain.QuotationsCommand{Version: 0, Reason: "Receive vendor quotation", Quote: purchaseDomain.QuotationsQuoteInput{DocumentID: document, Reference: "VENDOR-Q1", AttachmentID: &file.ID, ReceivedDate: today, ValidUntil: today, Currency: "INR", Freight: "0.01", OtherCharges: "0", DeliveryDays: 10, DeliveryTerms: "Delivered to site", PaymentTerms: "Against receipt", Lines: []purchaseDomain.QuotationsLineInput{{LineID: lineID, UnitPrice: &price, PricingMode: "unit_price", CoverageType: "both", WarrantyMonths: &months, Discount: "0", Tax: "0", Compliant: true}}}}
	if direct {
		amount := "123.46"
		command.Quote.Lines[0].PricingMode = "line_total"
		command.Quote.Lines[0].UnitPrice = nil
		command.Quote.Lines[0].LineTotal = &amount
	}
	if _, e = build(func(database.Connection) events.Writer { return purchaseRejectJournal{} }).Execute(ctx, "quote-writer", f.project, boID, command); e == nil {
		t.Fatal("quotation journal failure committed")
	}
	w, e := service.Execute(ctx, "quote-writer", f.project, boID, command)
	if e != nil || w.Record == nil || w.Record.Version != 1 || len(w.Revisions) != 1 || len(w.Comparisons) != 1 || !w.Comparisons[0].Eligible {
		t.Fatal("quotation receipt", e)
	}
	if w.Record.Quotes[0].Calculation.Total != wantTotal {
		t.Fatal("native exact total", w.Record.Quotes[0].Calculation.Total)
	}

	var storedTotal string
	if e = f.pool.QueryRow(ctx, "SELECT invoice_total::text FROM purchase_quotation_revisions WHERE document_id=$1 AND revision=1", document).Scan(&storedTotal); e != nil || storedTotal != wantTotal {
		t.Fatal("persisted invoice total", storedTotal, e)
	}
	if w.Record.Quotes[0].Lines[0].Tax != "18.00" {
		t.Fatal("central tax not snapshotted")
	}
	if _, e = service.Execute(ctx, "quote-writer", f.project, boID, command); !errors.Is(e, purchaseDomain.QuotationsStale) {
		t.Fatal("stale quote accepted", e)
	}
	command.Version = 1
	if _, e = service.Execute(ctx, "quote-writer", f.project, boID, command); e == nil {
		t.Fatal("reused evidence accepted for revision")
	}
	var multipartBody bytes.Buffer
	multi := multipart.NewWriter(&multipartBody)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="revised.pdf"`)
	header.Set("Content-Type", "application/pdf")
	part, e := multi.CreatePart(header)
	if e != nil {
		t.Fatal(e)
	}
	_, _ = part.Write(append(data, []byte("\n")...))
	_ = multi.Close()
	uploadRequest := httptest.NewRequest("POST", base+"/"+document+"/attachments", &multipartBody)
	uploadRequest.Header.Set("Content-Type", multi.FormDataContentType())
	uploadRequest.Header.Set("Authorization", "Bearer quote-writer")
	uploadResponse := httptest.NewRecorder()
	router.ServeHTTP(uploadResponse, uploadRequest)
	var file2 purchaseDomain.QuotationsAttachment
	if uploadResponse.Code != 201 || json.Unmarshal(uploadResponse.Body.Bytes(), &file2) != nil {
		t.Fatal("authorized multipart upload", uploadResponse.Code, uploadResponse.Body.String())
	}
	command.Quote.AttachmentID = &file2.ID
	command.Reason = "Receive revised evidence"
	w, e = service.Execute(ctx, "quote-writer", f.project, boID, command)
	if e != nil || len(w.Revisions) != 2 || w.Record.Quotes[0].Revision != 2 {
		t.Fatal("revision preserved", e)
	}
	report, e := service.Workspace(ctx, "comparison-reader", f.project, boID, "comparison")
	if e != nil || report.Record != nil || len(report.Attachments) != 0 || len(report.History) != 0 || len(report.Revisions) != 0 || len(report.Comparisons) != 1 || len(report.ComparisonRevisions) != 2 || report.CanWrite {
		t.Fatal("comparison authority leaked receipt details", e)
	}
	if _, _, e = service.Download(ctx, "comparison-reader", f.project, boID, document, file.ID); !errors.Is(e, purchaseDomain.QuotationsForbidden) {
		t.Fatal("comparison reader downloaded receipt evidence", e)
	}
	// Finance consumes the owning quotation publication, never browser totals or
	// a sibling persistence adapter. No financial decision is made by this reader.
	if _, e = purchaseRepository.NewQuotationsApprovalReader(f.pool, factory(f.pool), purchaseRepository.NewIndentGroupsReader(f.pool), purchaseRepository.Values{}).Source(ctx, f.project, boID); e == nil {
		t.Fatal("approval source accepted a pool outside the shared transaction")
	}
	e = database.NewManager(f.pool).Within(ctx, func(txCtx context.Context, c database.Connection) error {
		reader := purchaseRepository.NewQuotationsApprovalReader(c, factory(c), purchaseRepository.NewIndentGroupsReader(c), purchaseRepository.Values{})
		if _, e := reader.Source(ctx, f.project, boID); e == nil {
			t.Fatal("approval source accepted the outer transaction context")
		}
		if _, e := purchaseRepository.NewQuotationsApprovalReader(f.pool, factory(c), purchaseRepository.NewIndentGroupsReader(c), purchaseRepository.Values{}).Source(txCtx, f.project, boID); e == nil {
			t.Fatal("approval source accepted another connection")
		}
		source, e := reader.Source(txCtx, f.project, boID)
		if e != nil {
			return e
		}
		if source.ProjectID != f.project || source.BOID != boID || source.Currency != "INR" || !source.Available || len(source.Quotes) != 2 || len(source.Groups) == 0 {
			t.Fatal("incomplete Finance quotation source")
		}
		for i, quote := range source.Quotes {
			if quote.Revision != int32(i+1) || quote.DocumentID != document || quote.Total != wantTotal || !quote.Eligible || len(quote.Lines) != 1 || quote.Lines[0].Landed == nil {
				t.Fatal("saved negotiated calculation/revision not retained")
			}
			found := false
			for _, group := range source.Groups {
				found = found || group.ID == quote.GroupID
			}
			if !found {
				t.Fatal("quotation is missing its current group")
			}
		}
		if _, e := reader.Source(txCtx, f.project, document); !errors.Is(e, purchaseDomain.QuotationsMissing) {
			t.Fatal("approval source accepted a different BO scope", e)
		}
		future := quoteSourceClock{now: purchaseRepository.Values{}.Now().Add(24 * time.Hour)}
		expired, e := purchaseRepository.NewQuotationsApprovalReader(c, factory(c), purchaseRepository.NewIndentGroupsReader(c), future).Source(txCtx, f.project, boID)
		if e != nil {
			return e
		}
		for _, quote := range expired.Quotes {
			if quote.Eligible || !quote.Expired || quote.Total != wantTotal {
				t.Fatal("revision eligibility was not re-evaluated against current date")
			}
		}
		return nil
	})
	if e != nil {
		t.Fatal("native Finance quotation source", e)
	}
	verifyNativeQuotationApproval(t, f, ids, boID, document, lineID, factory, direct, allocationMode)
	for _, sql := range []string{"UPDATE purchase_quotation_revisions SET quote='{}'", "DELETE FROM purchase_quotation_attachments", "UPDATE purchase_quotation_events SET reason='rewritten'", "DELETE FROM purchase_quotation_evidence_events"} {
		if _, e = f.pool.Exec(ctx, sql); e == nil {
			t.Fatal("immutable evidence changed", sql)
		}
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_quotation_attachments").Scan(&count); e != nil || count != 2 {
		t.Fatal("evidence rollback failed", count, e)
	}
	migration, e := schema.Open(ctx, f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer migration.Close()
	if e = migration.Steps(-(schema.LatestVersion - 35)); e == nil {
		t.Fatal("populated quotation migration rollback allowed")
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM purchase_quotation_attachments").Scan(&count); e != nil || count != 2 {
		t.Fatal("rollback guard lost quotation evidence", e)
	}
}

type quoteSourceClock struct {
	purchaseRepository.Values
	now time.Time
}

func (v quoteSourceClock) Now() time.Time { return v.now }

type quoteReadProbe struct{ read bool }

func (p *quoteReadProbe) Read(_ []byte) (int, error) { p.read = true; return 0, io.EOF }
