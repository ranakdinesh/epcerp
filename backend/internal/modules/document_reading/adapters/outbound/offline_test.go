package outbound

import (
	"context"
	"github.com/xuri/excelize/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This explicit Docker gate exercises the production child binary/tools offline;
// fixtures contain synthetic text only and never enter an operational database.
func TestOfflineProductionReaderFormats(t *testing.T) {
	if os.Getenv("EPCERP_READER_TOOLS_TEST") != "1" {
		t.Skip("run scripts/verify-document-reader.sh with the built isolated reader image")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	key, e := NewCipher(strings.Repeat("ab", 32))
	if e != nil {
		t.Fatal(e)
	}
	engine := ProcessEngine{Binary: "/app/document-reader"}
	cases := []struct{ name, format, want, method string }{{"native.pdf", "pdf", "TENDER DOCUMENT READING FIXTURE", "native"}, {"scanned.pdf", "pdf", "SCANNED TENDER NOTICE", "ocr"}, {"native.docx", "docx", "Tender source document fixture", "native"}, {"scanned.docx", "docx", "SCANNED TENDER NOTICE", "ocr"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, e := os.ReadFile(filepath.Join("testdata", c.name))
			if e != nil {
				t.Fatal(e)
			}
			r, e := engine.Read(ctx, b, c.format, key.Hash(b))
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, s := range r.Segments {
				if strings.Contains(strings.ToUpper(s.Text), strings.ToUpper(c.want)) && s.Method == c.method {
					found = true
					if c.method == "ocr" && (s.Confidence == nil || len(s.Words) == 0 || s.CoordinateUnit != "pixels") {
						t.Fatal("OCR confidence/location missing")
					}
				}
			}
			if !found {
				t.Fatal("expected reading/method missing", c.name)
			}
		})
	}
	t.Run("legacy_office", func(t *testing.T) {
		dir := t.TempDir()
		b, e := os.ReadFile("testdata/native.docx")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "source.docx"), b, 0600); e != nil {
			t.Fatal(e)
		}
		f := excelize.NewFile()
		f.SetCellValue("Sheet1", "A1", "Legacy Excel tender fixture")
		if e = f.SaveAs(filepath.Join(dir, "workbook.xlsx")); e != nil {
			t.Fatal(e)
		}
		f.Close()
		profile := filepath.Join(dir, "profile")
		for _, c := range []struct{ input, filter, output, format, want string }{{"source.docx", "doc:MS Word 97", "source.doc", "doc", "Tender source document fixture"}, {"workbook.xlsx", "xls:MS Excel 97", "workbook.xls", "xls", "Legacy Excel tender fixture"}} {
			if _, e = command(ctx, 1<<20, "soffice", "-env:UserInstallation=file://"+profile, "--headless", "--convert-to", c.filter, "--outdir", dir, filepath.Join(dir, c.input)); e != nil {
				t.Fatal("fixture conversion", e)
			}
			data, e := os.ReadFile(filepath.Join(dir, c.output))
			if e != nil {
				t.Fatal(e)
			}
			r, e := engine.Read(ctx, data, c.format, key.Hash(data))
			if e != nil {
				t.Fatal("legacy read", e)
			}
			found := false
			for _, s := range r.Segments {
				if strings.Contains(s.Text, c.want) {
					found = true
				}
			}
			if !found {
				t.Fatal("legacy source text lost", c.format)
			}
		}
	})
}
