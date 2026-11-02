package outbound

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNamedFormatDOCXUsesFrozenVariableQuestionSet(t *testing.T) {
	for _, n := range []int{1, 3, 71} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			in, e := TSCRegistry()
			if e != nil {
				t.Fatal(e)
			}
			in.FormatID = domain.DefaultTSCFormatID
			in.FormatName = "Synthetic equipment supply & works TSC"
			in.FormatVersion = 3
			in.Name = "Synthetic test tender"
			in.Questions = nil
			result := domain.TSCResult{}
			for i := range n {
				id := "custom-" + strconv.Itoa(i)
				in.Questions = append(in.Questions, domain.TSCQuestion{ID: id, Section: "Commercial", Text: "Synthetic question <" + strconv.Itoa(i) + ">"})
				result.Answers = append(result.Answers, domain.TSCAnswer{QuestionID: id, Status: "answered", Text: "INR 5000"})
			}
			data, e := (TSCExporter{}).Export(domain.TSCJob{Input: in, Result: result})
			if e != nil {
				t.Fatal(e)
			}
			reader, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if e != nil {
				t.Fatal(e)
			}
			for _, f := range reader.File {
				if f.Name != "word/document.xml" {
					continue
				}
				r, _ := f.Open()
				raw, _ := io.ReadAll(r)
				r.Close()
				text := string(raw)
				if len(tscRows.FindAllString(text, -1)) != n+1 || strings.Count(text, "INR 5000") != n || !strings.Contains(text, "Synthetic equipment supply &amp; works TSC") {
					t.Fatal("export ignored custom snapshot")
				}
				decoder := xml.NewDecoder(bytes.NewReader(raw))
				for {
					_, e := decoder.Token()
					if e == io.EOF {
						break
					}
					if e != nil {
						t.Fatal(e)
					}
				}
			}
			if dir := os.Getenv("EPCERP_TSC_EXPORT_PROOF"); dir != "" && n == 3 {
				if e = os.WriteFile(filepath.Join(dir, "synthetic-named-tsc.docx"), data, 0600); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}
