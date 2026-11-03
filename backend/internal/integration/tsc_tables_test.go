package integration

import (
	"bytes"
	"context"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	tenderout "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
	"testing"
)

type syntheticPDFLayout struct{ calls int }

func (l *syntheticPDFLayout) Extract(_ context.Context, original []byte) (reading.PDFTables, error) {
	l.calls++
	if !bytes.HasPrefix(original, []byte("%PDF-")) {
		panic("authorized original PDF was not supplied")
	}
	return reading.PDFTables{Version: "pdf-tables-v1", Pages: []reading.PageTables{{Page: 1, Width: 600, Height: 800, Tables: []reading.PDFTable{{ID: "p1-t1", Cells: []reading.TableCell{{Row: 0, Column: 0, RowSpan: 1, ColumnSpan: 1, Text: "Validity of Offer (Days)", X: 10, Y: 10, Width: 100, Height: 20}, {Row: 0, Column: 1, RowSpan: 1, ColumnSpan: 1, Text: "60", X: 110, Y: 10, Width: 100, Height: 20}}}}}}}, nil
}
func verifyTSCLegacyTableSnapshot(t *testing.T, ctx context.Context, unit *tenderout.TSCUnit, draft domain.DocumentDraft, owner string) {
	t.Helper()
	layout := &syntheticPDFLayout{}
	unit.WithLayouts(layout)
	defer unit.WithLayouts(nil)
	if err := unit.Within(ctx, func(ctx context.Context, r ports.TSCRepository, d ports.DraftRepository) error {
		current, e := d.Get(ctx, draft.ID, owner)
		if e != nil {
			return e
		}
		// Snapshot populates authorized package sources; historical reading bytes remain intact.
		input, e := r.Snapshot(ctx, current)
		if e != nil {
			return e
		}
		if layout.calls < 1 {
			t.Fatal("legacy reading was not enriched")
		}
		facts := domain.CollectTSCCandidates(input)
		if facts["particulars.q14"].Text != "60 days" {
			t.Fatal("enriched cells not used", facts)
		}
		source := input.Sources[0].Source.ID
		before, e := d.Artifact(ctx, current, source, "result")
		if e != nil {
			return e
		}
		_, e = r.Snapshot(ctx, current)
		if e != nil {
			return e
		}
		after, e := d.Artifact(ctx, current, source, "result")
		if e != nil {
			return e
		}
		if !bytes.Equal(before, after) {
			t.Fatal("legacy source reading was rewritten")
		}
		return nil
	}); err != nil {
		t.Fatal("native table enrichment", err)
	}
}
