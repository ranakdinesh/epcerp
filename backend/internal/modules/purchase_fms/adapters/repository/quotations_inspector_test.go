package repository

import (
	"archive/zip"
	"bytes"
	"testing"
)

func quotationsWorkbook(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for n, s := range entries {
		w, e := z.Create(n)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write([]byte(s)); e != nil {
			t.Fatal(e)
		}
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestQuotationsQuotationEvidenceTypes(t *testing.T) {
	xlsx := quotationsWorkbook(t, map[string]string{"[Content_Types].xml": "<Types/>", "xl/workbook.xml": "<workbook/>"})
	for _, tc := range []struct {
		name, mime string
		data       []byte
		valid      bool
	}{
		{"quote.pdf", "application/pdf", []byte("%PDF-1.7\n%%EOF"), true},
		{"quote.pdf", "image/png", []byte("%PDF-1.7\n%%EOF"), false},
		{"quote.pdf", "application/pdf", []byte("<html>not PDF</html>"), false},
		{"../quote.pdf", "application/pdf", []byte("%PDF-1.7\n%%EOF"), false},
		{"quote.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", xlsx, true},
		{"quote.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []byte("PKfake"), false},
		{"quote.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", quotationsWorkbook(t, map[string]string{"[Content_Types].xml": "<Types/>"}), false},
		{"quote.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", quotationsWorkbook(t, map[string]string{"[Content_Types].xml": "<Types/>", "xl/workbook.xml": "<workbook/>", "xl/vbaProject.bin": "macro"}), false},
		{"quote.exe", "application/octet-stream", []byte("binary"), false},
	} {
		t.Run(tc.name+tc.mime, func(t *testing.T) {
			e := (QuotationsInspector{}).Inspect(tc.name, tc.mime, tc.data)
			if (e == nil) != tc.valid {
				t.Fatalf("valid=%t err=%v", tc.valid, e)
			}
		})
	}
}
