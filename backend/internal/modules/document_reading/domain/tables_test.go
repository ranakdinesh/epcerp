package domain

import (
	"math"
	"strings"
	"testing"
)

func TestTableEvidencePreservesMultiColumnSpansAndCompleteWrappedCells(t *testing.T) {
	scope := strings.Repeat("Synthetic scope line with all conditions.\n", 45)
	table := PDFTable{ID: "p1-t1", Cells: []TableCell{{Row: 0, Column: 0, RowSpan: 1, ColumnSpan: 1, Text: "Name of work"}, {Row: 0, Column: 1, RowSpan: 1, ColumnSpan: 3, Text: scope}, {Row: 1, Column: 0, RowSpan: 1, ColumnSpan: 1, Text: "EMD"}, {Row: 1, Column: 1, RowSpan: 1, ColumnSpan: 1, Text: "250000"}, {Row: 1, Column: 2, RowSpan: 1, ColumnSpan: 1, Text: "Offer validity (Days)"}, {Row: 1, Column: 3, RowSpan: 1, ColumnSpan: 1, Text: "45"}}}
	r := Result{Segments: []Segment{{Kind: "page", Label: "Page 1", Method: "native", Text: "Name of work     only a wrong tail\nEMD  250000  Offer validity (Days)  45", Tables: []PDFTable{table}}}}
	e := BuildEvidence(r)
	rows := []EvidenceBlock{}
	for _, b := range e.Blocks {
		if b.Kind == "table_row" {
			rows = append(rows, b)
		} else if b.Kind == "table" && !b.Suppressed {
			t.Fatal("lossy duplicate table remains primary")
		}
	}
	if len(rows) != 2 || len(rows[0].TableCells) != 2 || rows[0].TableCells[1].ColumnSpan != 3 || !strings.Contains(rows[0].Text, strings.TrimSpace(scope)) {
		t.Fatal("wrapped or merged source cell lost")
	}
	if len(rows[1].TableCells) != 4 || rows[1].TableCells[1].Text != "250000" || rows[1].TableCells[3].Text != "45" {
		t.Fatal("paired rows conflated")
	}
	if r.Segments[0].Text != "Name of work     only a wrong tail\nEMD  250000  Offer validity (Days)  45" {
		t.Fatal("original text overwritten")
	}
}
func TestTableGeometryRejectsOutOfPageOrNonFiniteCoordinates(t *testing.T) {
	for _, bad := range []float64{-1, math.NaN(), math.Inf(1), 700} {
		t.Run("bounds", func(t *testing.T) {
			v := PDFTables{Version: "pdf-tables-v1", Pages: []PageTables{{Page: 1, Width: 600, Height: 800, Tables: []PDFTable{{ID: "x", Cells: []TableCell{{RowSpan: 1, ColumnSpan: 1, X: bad, Y: 10, Width: 20, Height: 20}}}}}}}
			if v.Validate() == nil {
				t.Fatal("unbounded geometry trusted")
			}
		})
	}
}
