package pdfreport

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestLandscapeTableLongCellsAndPageContinuation(t *testing.T) {
	d := Document{Title: "Items to purchase", Subtitle: "PRJ-01 - Long project name", Stamp: "Generated 12 Oct 2026", Columns: []Column{{"Item / SKU", 200}, {"Vendor with a long wrapped comparison heading\nQuotation revision 2", 593}}, Rows: [][]string{{strings.Repeat("ABC-123-", 120), strings.Repeat("Technical specifications with Unicode é Δ · and dimensions. ", 180)}}, Notes: []string{strings.Repeat("Finalisation comment retained. ", 120)}}
	v, e := Render(context.Background(), d)
	if e != nil || !bytes.HasPrefix(v, []byte("%PDF-")) {
		t.Fatal(e)
	}
	if bytes.Count(v, []byte("/Type /Page")) < 3 {
		t.Fatal("long row did not paginate")
	}
	if path := os.Getenv("EPCERP_PDF_PREVIEW"); path != "" {
		if e := os.WriteFile(path, v, 0600); e != nil {
			t.Fatal(e)
		}
	}
	d.Rows = [][]string{{"Item", ""}}
	if _, err := Render(context.Background(), d); err != nil {
		t.Fatal("optional blank cell", err)
	}
	d.Columns[1].Width = 800
	if _, e := Render(context.Background(), d); e == nil {
		t.Fatal("overflow accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d.Columns[1].Width = 593
	if _, e := Render(ctx, d); e == nil {
		t.Fatal("cancelled render accepted")
	}
}
