package outbound

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	"github.com/xuri/excelize/v2"
	"strings"
	"testing"
)

func wordFixture(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, e := z.Create("word/document.xml")
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.Write([]byte(`<w:document xmlns:w="urn:word"><w:body><w:p><w:r><w:t>Tender notice</w:t></w:r></w:p><w:tbl><w:tr><w:tc><w:p><w:r><w:t>Cables</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>100 meters</w:t></w:r></w:p></w:tc></w:tr></w:tbl></w:body></w:document>`))
	if e != nil {
		t.Fatal(e)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestWordNativeReadingAndOriginalIntegrity(t *testing.T) {
	data := wordFixture(t)
	c, e := NewCipher(strings.Repeat("ab", 32))
	if e != nil {
		t.Fatal(e)
	}
	sealed, hash, e := c.Seal(data)
	if e != nil || bytes.Equal(sealed, data) {
		t.Fatal("encryption", e)
	}
	plain, e := c.Open(sealed, hash)
	if e != nil || !bytes.Equal(plain, data) {
		t.Fatal("integrity", e)
	}
	sealed[len(sealed)-1] ^= 1
	if _, e = c.Open(sealed, hash); e == nil {
		t.Fatal("tampering accepted")
	}
	r, e := (Parser{}).Read(context.Background(), data, "docx", hash)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Segments) != 1 || !strings.Contains(r.Segments[0].Text, "Cables") || !strings.Contains(r.Segments[0].Text, "100 meters") {
		t.Fatal("paragraph/table reading", r)
	}
	encoded, e := (Codec{}).Encode(r)
	if e != nil {
		t.Fatal(e)
	}
	text, e := (Codec{}).Text(encoded)
	if e != nil || !bytes.Contains(text, []byte("Tender notice")) {
		t.Fatal("text artifact", e)
	}
}
func TestExcelSheetsCellsFormulasAndMerges(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	if e := f.SetCellValue("Sheet1", "A1", "Tender BOQ"); e != nil {
		t.Fatal(e)
	}
	f.SetCellValue("Sheet1", "B3", 125.5)
	f.SetCellFormula("Sheet1", "C3", "B3*2")
	f.MergeCell("Sheet1", "A1", "C1")
	f.NewSheet("Hidden")
	f.SetCellValue("Hidden", "D4", "Supporting source")
	f.SetSheetVisible("Hidden", false)
	b, e := f.WriteToBuffer()
	if e != nil {
		t.Fatal(e)
	}
	r, e := (Parser{}).Read(context.Background(), b.Bytes(), "xlsx", strings.Repeat("0", 64))
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Segments) != 2 || len(r.Segments[0].Merges) != 1 {
		t.Fatal("sheets/merges")
	}
	formula := false
	for _, c := range r.Segments[0].Cells {
		if c.Address == "C3" && c.Formula == "B3*2" {
			formula = true
		}
	}
	if !formula {
		t.Fatal("formula lost", r.Segments[0].Cells)
	}
	if !strings.Contains(r.Segments[1].Text, "Supporting source") {
		t.Fatal("hidden sheet skipped")
	}
}
func TestSpoofedArchivesAndOutputLimits(t *testing.T) {
	for _, format := range []string{"pdf", "docx", "xlsx", "xls", "doc"} {
		if e := (Inspector{}).Inspect([]byte("not a document"), format); e == nil {
			t.Fatal("spoof accepted", format)
		}
	}
	r := domain.Result{Schema: domain.ReaderVersion, ReaderVersion: domain.ReaderVersion, Segments: []domain.Segment{{Text: strings.Repeat("x", 8<<20)}}}
	if _, e := (Codec{}).Encode(r); e == nil {
		t.Fatal("oversized result accepted")
	}
}

func TestArchiveExpansionMacrosAndDuplicateEntries(t *testing.T) {
	for _, kind := range []string{"expansion", "macro", "duplicate"} {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		header := &zip.FileHeader{Name: "word/document.xml", Method: zip.Store, UncompressedSize64: 1, CompressedSize64: 1}
		if kind == "expansion" {
			header.UncompressedSize64 = 257 << 20
		}
		w, e := z.CreateRaw(header)
		if e != nil {
			t.Fatal(e)
		}
		w.Write([]byte("x"))
		if kind == "macro" {
			w, e = z.Create("word/vbaProject.bin")
			if e != nil {
				t.Fatal(e)
			}
			w.Write([]byte("x"))
		}
		if kind == "duplicate" {
			w, e = z.Create("word/document.xml")
			if e != nil {
				t.Fatal(e)
			}
			w.Write([]byte("x"))
		}
		if e = z.Close(); e != nil {
			t.Fatal(e)
		}
		if e := (Inspector{}).Inspect(b.Bytes(), "docx"); e == nil {
			t.Fatal("unsafe archive accepted", kind)
		}
	}
}

func TestLegacyConversionDisablesAutomaticExternalUpdates(t *testing.T) {
	var profile struct {
		Items []struct {
			Path  string `xml:"path,attr"`
			Props []struct {
				Name  string `xml:"name,attr"`
				Value string `xml:"value"`
			} `xml:"prop"`
		} `xml:"item"`
	}
	if err := xml.Unmarshal([]byte(legacyOfficeProfile), &profile); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, item := range profile.Items {
		for _, prop := range item.Props {
			got[item.Path+"/"+prop.Name] = prop.Value
		}
	}
	for key, want := range map[string]string{
		"/org.openoffice.Office.Common/Security/Scripting/MacroSecurityLevel": "3",
		"/org.openoffice.Office.Calc/Content/Update/Link":                     "1",
		"/org.openoffice.Office.Writer/Content/Update/Link":                   "2",
		"/org.openoffice.Office.Writer/Content/Update/Field":                  "false",
		"/org.openoffice.Office.Writer/Content/Update/Chart":                  "false",
	} {
		if got[key] != want {
			t.Errorf("%s: got %q, want %q", key, got[key], want)
		}
	}
}
