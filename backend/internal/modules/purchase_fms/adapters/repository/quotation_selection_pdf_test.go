package repository

import (
	"bytes"
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"os"
	"strings"
	"testing"
	"time"
)

func selectionPrintFixture() domain.QuotationApprovalsPrint {
	return domain.QuotationApprovalsPrint{ProjectNumber: "PRJ-QA", ProjectName: "Selection PDF verification", GroupName: "Electrical package", Currency: "INR", GeneratedAt: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC), Quotes: []domain.QuotationApprovalsQuote{
		{Key: "one:1", Vendor: "Lowest-cost supplier", Reference: "Q-01", Revision: 1, Eligible: true, Lowest: true, Total: "1000.00", Lines: []domain.QuotationApprovalsQuoteLine{{LineID: "line", Item: domain.QuotationApprovalsItem{Description: "Cable specification with controlled engineering attributes", SKU: "CABLE-001", UOM: "m"}, Quantity: "10", Quoted: "1000.00"}}},
		{Key: "two:1", Vendor: "Preferred supplier", Reference: "Q-02", Revision: 1, Eligible: true, Total: "1100.00", Lines: []domain.QuotationApprovalsQuoteLine{{LineID: "line", Item: domain.QuotationApprovalsItem{Description: "Cable", SKU: "CABLE-001", UOM: "m"}, Quantity: "10", Quoted: "1100.00"}}},
	}, Nominations: []domain.QuotationApprovalsNomination{{QuoteKey: "one:1", BestPrice: true, BestPriceComment: "Lowest complete landed cost"}, {QuoteKey: "two:1", Preferred: true, PreferredComment: "Earlier delivery and longer warranty"}}}
}
func TestSelectionPDFHeadingsAndIndependentOfferColours(t *testing.T) {
	s := selectionPrintFixture()
	sheet, e := quotationComparisonSheet(s)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(sheet.Columns[2].Title, "Lowest price") || !strings.Contains(sheet.Columns[3].Title, "\nPreferred") {
		t.Fatal("selection hidden in footnotes", sheet.Columns)
	}
	if len(sheet.ColumnFills[2].Colors) != 1 || len(sheet.ColumnFills[3].Colors) != 1 || sheet.ColumnFills[2].Colors[0] == sheet.ColumnFills[3].Colors[0] {
		t.Fatal("lowest/preferred colours missing")
	}
	if sheet.Rows[len(sheet.Rows)-1][2] != "INR 1000.00" || sheet.Rows[len(sheet.Rows)-1][3] != "INR 1100.00" {
		t.Fatal("presentation changed totals")
	}
	s.Nominations = nil
	sheet, e = quotationComparisonSheet(s)
	if e != nil || !strings.Contains(sheet.Columns[2].Title, "Lowest price") || strings.Contains(sheet.Columns[3].Title, "\nPreferred") {
		t.Fatal("comparison lost computed lowest or invented preference", e)
	}
}
func TestSameOfferLowestAndPreferredFrozenIdentityAndContinuation(t *testing.T) {
	s := selectionPrintFixture()
	n := domain.QuotationApprovalsNomination{QuoteKey: "one:1", BestPrice: true, Preferred: true, BestPriceComment: "Lowest complete landed cost", PreferredComment: "Preferred delivery and warranty"}
	snapshot := domain.QuotationApprovalsAttempt{Vendor: "Selected supplier", Reference: "Q-01", Revision: 1, Total: "1000.00", Lines: []domain.QuotationApprovalsLine{}}
	for i := 0; i < 40; i++ {
		snapshot.Lines = append(snapshot.Lines, domain.QuotationApprovalsLine{LineID: string(rune('A' + i)), Item: domain.QuotationApprovalsItem{Description: strings.Repeat("Long engineering specification ", 4), SKU: "SKU-001", UOM: "m"}, Quantity: "10", Quoted: "25.00"})
	}
	// Historical snapshot may omit QuoteKey; the persisted proposal key remains authoritative.
	s.Attempt = &domain.QuotationApprovalsAttempt{ID: "submission", State: "pending_accounts", EffectiveState: "pending_accounts", SubmittedAt: s.GeneratedAt, Proposals: []domain.QuotationApprovalsProposal{{QuotationApprovalsNomination: n, Snapshot: snapshot}}}
	sheet, e := quotationComparisonSheet(s)
	if e != nil {
		t.Fatal(e)
	}
	if len(sheet.ColumnFills[2].Colors) != 2 || !strings.Contains(sheet.Columns[2].Title, "Lowest price + Preferred") {
		t.Fatal("dual designation or frozen identity lost", sheet.Columns)
	}
	data, e := (QuotationApprovalPrinter{}).Render(context.Background(), s)
	if e != nil || !bytes.HasPrefix(data, []byte("%PDF-")) || bytes.Count(data, []byte("/Type /Page")) < 3 {
		t.Fatal("continued selection PDF invalid", e)
	}
	if path := os.Getenv("EPCERP_QUOTATION_PDF_PREVIEW"); path != "" {
		if e = os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
