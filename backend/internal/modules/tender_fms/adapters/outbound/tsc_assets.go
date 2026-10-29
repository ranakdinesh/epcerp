package outbound

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"io"
	"regexp"
	"strconv"
	"strings"
)

//go:embed tscassets/questions.json tscassets/template.docx
var tscAssets embed.FS

func TSCRegistry() (domain.TSCInput, error) {
	raw, e := tscAssets.ReadFile("tscassets/questions.json")
	if e != nil {
		return domain.TSCInput{}, e
	}
	var v struct {
		Questions  []domain.TSCQuestion `json:"questions"`
		SourceHash string               `json:"template_sha256"`
	}
	if e = json.Unmarshal(raw, &v); e != nil {
		return domain.TSCInput{}, e
	}
	if len(v.Questions) != 70 {
		return domain.TSCInput{}, errors.New("Invalid TSC registry")
	}
	template, e := tscAssets.ReadFile("tscassets/template.docx")
	if e != nil {
		return domain.TSCInput{}, e
	}
	sum := sha256.Sum256(template)
	return domain.TSCInput{Schema: domain.TSCVersion, TemplateHash: hex.EncodeToString(sum[:]), OriginalTemplateHash: v.SourceHash, Questions: v.Questions, Passages: []domain.TSCPassage{}, Warnings: []string{"Draft only: no company capability register is connected; company eligibility and bid decisions require manual review.", "Reading success is not package completeness. Retrieval proposals may miss clauses; verify against original sources.", "BOQ item/make extraction and final approval are not included in this draft slice."}}, nil
}

type TSCExporter struct{}

func fmtInt(n int64) string { return strconv.FormatInt(n, 10) }
func tscParagraph(s string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(s))
	return `<w:p><w:r><w:rPr><w:sz w:val="20"/></w:rPr><w:t xml:space="preserve">` + escaped.String() + `</w:t></w:r></w:p>`
}

var tscRows = regexp.MustCompile(`(?s)<w:tr\b[^>]*>.*?</w:tr>`)
var tscCells = regexp.MustCompile(`(?s)<w:tc\b[^>]*>.*?</w:tc>`)
var tscCellProps = regexp.MustCompile(`(?s)^<w:tc\b[^>]*>(?:<w:tcPr>.*?</w:tcPr>)?`)

func (TSCExporter) Export(j domain.TSCJob) ([]byte, error) {
	template, e := tscAssets.ReadFile("tscassets/template.docx")
	if e != nil {
		return nil, e
	}
	sum := sha256.Sum256(template)
	if hex.EncodeToString(sum[:]) != j.Input.TemplateHash {
		return nil, errors.New("TSC template revision mismatch")
	}
	reader, e := zip.NewReader(bytes.NewReader(template), int64(len(template)))
	if e != nil {
		return nil, e
	}
	answers := map[string]domain.TSCAnswer{}
	for _, a := range j.Result.Answers {
		answers[a.QuestionID] = a
	}
	rowAnswers := map[int]string{}
	for _, q := range j.Input.Questions {
		a, ok := answers[q.ID]
		if !ok {
			return nil, errors.New("Incomplete TSC snapshot")
		}
		text := tscChecklistAnswer(a)
		rowAnswers[q.Row] = text
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, file := range reader.File {
		r, e := file.Open()
		if e != nil {
			return nil, e
		}
		data, e := io.ReadAll(io.LimitReader(r, 8<<20))
		r.Close()
		if e != nil {
			return nil, e
		}
		if file.Name == "word/document.xml" {
			row := 0
			filled := 0
			xml := tscRows.ReplaceAllStringFunc(string(data), func(s string) string {
				index := row
				row++
				text, ok := rowAnswers[index]
				if index == 1 {
					text = "TENDER SELECTION CHECK LIST — DRAFT — " + j.Input.Name
					ok = true
				}
				if index >= 77 && index <= 84 {
					text = ""
					ok = true
				}
				if !ok {
					return s
				}
				cells := tscCells.FindAllStringIndex(s, -1)
				if len(cells) == 0 {
					return s
				}
				last := cells[len(cells)-1]
				prefix := tscCellProps.FindString(s[last[0]:last[1]])
				if prefix == "" {
					return s
				}
				if _, question := rowAnswers[index]; question {
					filled++
				}
				return s[:last[0]] + prefix + tscParagraph(text) + "</w:tc>" + s[last[1]:]
			})
			if filled != 70 {
				return nil, errors.New("Template question coverage mismatch")
			}
			xml = tscRows.ReplaceAllStringFunc(xml, func(row string) string {
				// Keep each question and its answer together across page breaks.
				if !strings.Contains(row, "<w:cantSplit") {
					if strings.Contains(row, "<w:trPr>") {
						row = strings.Replace(row, "<w:trPr>", "<w:trPr><w:cantSplit/>", 1)
					} else if end := strings.Index(row, ">"); end >= 0 {
						row = row[:end+1] + "<w:trPr><w:cantSplit/></w:trPr>" + row[end+1:]
					}
				}
				// Merged section headings travel with their first question.
				if len(tscCells.FindAllStringIndex(row, -1)) == 1 {
					row = strings.ReplaceAll(row, "<w:pPr>", "<w:pPr><w:keepNext/>")
					row = strings.ReplaceAll(row, "<w:p><w:r>", "<w:p><w:pPr><w:keepNext/></w:pPr><w:r>")
				}
				return row
			})
			data = []byte(xml)
		}
		dst, e := writer.Create(file.Name)
		if e != nil {
			return nil, e
		}
		if _, e = dst.Write(data); e != nil {
			return nil, e
		}
	}
	if e = writer.Close(); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}

func tscChecklistAnswer(a domain.TSCAnswer) string {
	a = domain.ConciseTSCAnswer(a)
	switch a.Status {
	case "answered":
		text := strings.TrimSpace(a.Text)
		if strings.HasPrefix(strings.ToLower(text), "answered:") {
			text = strings.TrimSpace(text[len("answered:"):])
		}
		if domain.TSCPlaceholder(text) {
			return ""
		}
		return text
	default:
		return ""
	}
}
