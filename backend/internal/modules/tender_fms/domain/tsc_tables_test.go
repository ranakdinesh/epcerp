package domain

import (
	"strings"
	"testing"
)

func TestStructuredFactsKeepPairsUnitsZeroAndCompleteScope(t *testing.T) {
	scope := strings.Repeat("Different synthetic stations and complete electrical scope. ", 30)
	q := []TSCQuestion{{ID: "particulars.q02"}, {ID: "particulars.q07"}, {ID: "particulars.q09"}, {ID: "particulars.q14"}, {ID: "particulars.q16"}, {ID: "emd.q05"}, {ID: "tender_fee.q05"}}
	pairs := [][]string{{"Name of work", scope}, {"Completion period", "9 Months", "Validity of offer ( Days)", "45"}, {"Tender Closing Date Time", "21/02/2027 14:30"}, {"Contract type", "Works - General", "Contract category", "Capital"}, {"Earnest Money (Rs.)", "250000.00"}, {"Tender Doc. Cost (Rs.)", "0.00"}}
	in := TSCInput{Questions: q}
	for row, values := range pairs {
		p := TSCPassage{ID: values[0], Kind: "table_row", Method: "native_table"}
		for col, text := range values {
			p.TableCells = append(p.TableCells, TSCTableCell{Row: row, Column: col, RowSpan: 1, ColumnSpan: 1, Text: text})
			p.Text += text + " | "
		}
		in.Passages = append(in.Passages, p)
	}
	got := CollectTSCCandidates(in)
	for id, want := range map[string]string{"particulars.q02": strings.TrimSpace(scope), "particulars.q07": "9 Months", "particulars.q09": "21/02/2027 14:30", "particulars.q14": "45 days", "particulars.q16": "Works - General", "emd.q05": "Rs. 250000.00", "tender_fee.q05": "Rs. 0.00"} {
		if got[id].Text != want || got[id].Accepted {
			t.Fatalf("%s: %q != %q", id, got[id].Text, want)
		}
	}
	for _, a := range got {
		if len(a.Citations) != 1 {
			t.Fatal("cell evidence untraceable")
		}
	}
}
func TestBareUnverifiedPDFTableCannotSupplyAutomaticFacts(t *testing.T) {
	in := factInput([]string{"particulars.q16"}, TSCPassage{ID: "raw", Filename: "unknown.pdf", Kind: "table", Method: "native", Text: "Contract type     Works General    Contract category Capital"})
	if len(CollectTSCCandidates(in)) != 0 {
		t.Fatal("unverified columns silently accepted")
	}
}
func TestAdvertisedValueDoesNotInventTaxBasis(t *testing.T) {
	in := factInput([]string{"particulars.q04", "particulars.q05"}, TSCPassage{ID: "raw", Kind: "table_row", Method: "native_table", Text: "Advertised Value | 890000", TableCells: []TSCTableCell{{Column: 0, ColumnSpan: 1, Text: "Advertised Value"}, {Column: 1, ColumnSpan: 1, Text: "890000"}}})
	if len(CollectTSCCandidates(in)) != 0 {
		t.Fatal("unspecified GST basis invented")
	}
}
func TestRequiredExceptionStillBlocksWhenItCannotFit(t *testing.T) {
	a := TSCPassage{ID: "a", DocumentID: "d", Segment: 0, Text: strings.Repeat("x", 8995)}
	b := TSCPassage{ID: "b", DocumentID: "d", Segment: 1, Text: "Exception: this condition is mandatory."}
	out, ok := SelectRankedTSC(TSCQuestion{ID: "emd.q05"}, []TSCPassage{a, b}, []TSCPassage{a}, []TSCRank{{ID: "a", Score: 1}})
	if !ok || !out.Diagnostic.ContextGap {
		t.Fatal("required context was treated as optional")
	}
}
