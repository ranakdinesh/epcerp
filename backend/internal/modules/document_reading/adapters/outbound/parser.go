package outbound

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	"github.com/xuri/excelize/v2"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Link-update enum values differ between Calc and Writer; use each registry's
// Never value. Preserve cached fields/charts while converting legacy originals.
const legacyOfficeProfile = `<?xml version="1.0" encoding="UTF-8"?>
<oor:items xmlns:oor="http://openoffice.org/2001/registry">
<item oor:path="/org.openoffice.Office.Common/Security/Scripting"><prop oor:name="MacroSecurityLevel" oor:op="fuse"><value>3</value></prop></item>
<item oor:path="/org.openoffice.Office.Calc/Content/Update"><prop oor:name="Link" oor:op="fuse"><value>1</value></prop></item>
<item oor:path="/org.openoffice.Office.Writer/Content/Update"><prop oor:name="Link" oor:op="fuse"><value>2</value></prop><prop oor:name="Field" oor:op="fuse"><value>false</value></prop><prop oor:name="Chart" oor:op="fuse"><value>false</value></prop></item>
</oor:items>`

type Parser struct{}

func (Parser) Read(ctx context.Context, data []byte, format, hash string) (domain.Result, error) {
	r := domain.Result{Schema: domain.ReaderVersion, ReaderVersion: "epcerp-reading-v3", SourceSHA256: hash, Segments: []domain.Segment{}, Warnings: []string{}}
	if e := (Inspector{}).Inspect(data, format); e != nil {
		return r, e
	}
	dir, e := os.MkdirTemp("", "epcerp-read-")
	if e != nil {
		return r, e
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "source."+format)
	if e = os.WriteFile(path, data, 0600); e != nil {
		return r, e
	}
	if format == "doc" || format == "xls" {
		converted := "docx"
		if format == "xls" {
			converted = "xlsx"
		}
		profile := filepath.Join(dir, "profile")
		if e = os.MkdirAll(filepath.Join(profile, "user"), 0700); e != nil {
			return r, e
		}
		if e = os.WriteFile(filepath.Join(profile, "user", "registrymodifications.xcu"), []byte(legacyOfficeProfile), 0600); e != nil {
			return r, e
		}
		if _, e = command(ctx, 1<<20, "soffice", "-env:UserInstallation=file://"+profile, "--headless", "--nologo", "--nodefault", "--norestore", "--convert-to", converted, "--outdir", dir, path); e != nil {
			return r, domain.Invalid("Legacy Office conversion failed; save as XLSX or DOCX and upload again")
		}
		path = filepath.Join(dir, "source."+converted)
		data, e = os.ReadFile(path)
		if e != nil || len(data) > domain.MaxBytes {
			return r, domain.Invalid("Legacy Office conversion exceeded limits or produced no readable file")
		}
		if e = (Inspector{}).Inspect(data, converted); e != nil {
			return r, e
		}
		r.Warnings = append(r.Warnings, "Legacy Office file converted locally; conversion may recalculate formulas. Review values and formatting against the original.")
		format = converted
	}
	switch format {
	case "xlsx":
		e = readExcel(ctx, data, &r)
	case "docx":
		e = readWord(ctx, data, &r)
	case "pdf":
		e = readPDF(ctx, path, dir, &r)
	default:
		e = domain.Invalid("Unsupported document format")
	}
	if e != nil {
		return r, e
	}
	if e = domain.CheckResult(r); e != nil {
		return r, e
	}
	return r, nil
}
func readExcel(ctx context.Context, data []byte, r *domain.Result) error {
	f, e := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 256 << 20, UnzipXMLSizeLimit: 16 << 20})
	if e != nil {
		return domain.Invalid("Excel file could not be opened; check damage or password protection")
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) > 100 {
		return domain.Invalid("Workbook exceeds 100 sheets; split the workbook")
	}
	count := 0
	for _, sheet := range sheets {
		if e = ctx.Err(); e != nil {
			return e
		}
		rows, e := f.Rows(sheet)
		if e != nil {
			return e
		}
		s := domain.Segment{Kind: "sheet", Label: sheet, Method: "native", Cells: []domain.Cell{}}
		var text strings.Builder
		row := 0
		for rows.Next() {
			row++
			if row > 100000 {
				rows.Close()
				return domain.Invalid("Workbook exceeds the row limit")
			}
			values, e := rows.Columns()
			if e != nil {
				rows.Close()
				return e
			}
			for col, value := range values {

				address, e := excelize.CoordinatesToCellName(col+1, row)
				if e != nil {
					rows.Close()
					return e
				}
				formula, e := f.GetCellFormula(sheet, address)
				if e != nil {
					rows.Close()
					return e
				}
				if value == "" && formula == "" {
					continue
				}
				count++
				if count > 100000 {
					rows.Close()
					return domain.Invalid("Workbook exceeds 100,000 populated cells")
				}
				s.Cells = append(s.Cells, domain.Cell{Address: address, Value: value, Formula: formula})
				fmt.Fprintf(&text, "%s\t%s\n", address, value)
				if text.Len() > 8<<20 {
					rows.Close()
					return domain.Invalid("Workbook reading exceeds the result limit")
				}
			}
		}
		if err := rows.Error(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		s.Text = text.String()
		merges, e := f.GetMergeCells(sheet)
		if e != nil {
			return e
		}
		for _, m := range merges {
			s.Merges = append(s.Merges, m.GetStartAxis()+":"+m.GetEndAxis())
		}
		visible, e := f.GetSheetVisible(sheet)
		if e != nil {
			return e
		}
		if !visible {
			r.Warnings = append(r.Warnings, "Hidden worksheet included: "+sheet)
		}
		r.Segments = append(r.Segments, s)
	}
	r.Warnings = append(r.Warnings, "Formula values are stored cached values from the workbook; formulas and external links are not executed.")
	inventoryOfficeParts(data, "xlsx", r)
	return embeddedImages(ctx, data, "xl/media/", r)
}
func readWord(ctx context.Context, data []byte, r *domain.Result) error {
	z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		return e
	}
	for _, f := range z.File {
		if f.Name != "word/document.xml" && !strings.HasPrefix(f.Name, "word/header") && !strings.HasPrefix(f.Name, "word/footer") && f.Name != "word/footnotes.xml" && f.Name != "word/endnotes.xml" && f.Name != "word/comments.xml" {
			continue
		}
		rc, e := f.Open()
		if e != nil {
			return e
		}
		decoder := xml.NewDecoder(io.LimitReader(rc, 32<<20))
		var text strings.Builder
		inText := false
		for {
			token, e := decoder.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				rc.Close()
				return domain.Invalid("Word document XML is damaged or exceeds reading limits")
			}
			if e = ctx.Err(); e != nil {
				rc.Close()
				return e
			}
			switch t := token.(type) {
			case xml.StartElement:
				if t.Name.Local == "t" || t.Name.Local == "delText" {
					inText = true
				}
				if t.Name.Local == "tab" {
					text.WriteString("\t")
				}
				if t.Name.Local == "br" {
					text.WriteString("\n")
				}
			case xml.EndElement:
				if t.Name.Local == "t" || t.Name.Local == "delText" {
					inText = false
				}
				if t.Name.Local == "p" || t.Name.Local == "tr" {
					text.WriteString("\n")
				}
				if t.Name.Local == "tc" {
					text.WriteString("\t")
				}
			case xml.CharData:
				if inText {
					text.Write(t)
				}
			}
			if text.Len() > 8<<20 {
				rc.Close()
				return domain.Invalid("Word reading exceeds the result limit")
			}
		}
		rc.Close()
		r.Segments = append(r.Segments, domain.Segment{Kind: "document", Label: f.Name, Text: text.String(), Method: "native"})
	}
	r.Warnings = append(r.Warnings, "Word body, headers, footers, footnotes, endnotes and comments are listed by source part. Tracked inserted/deleted text may coexist; review revisions in the original. Native Word text has part/paragraph order, not rendered page coordinates.")
	inventoryOfficeParts(data, "docx", r)
	return embeddedImages(ctx, data, "word/media/", r)
}
func embeddedImages(ctx context.Context, data []byte, prefix string, r *domain.Result) error {
	z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		return e
	}
	dir, e := os.MkdirTemp("", "epcerp-images-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	count := 0
	for _, f := range z.File {
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".tif" && ext != ".tiff" && ext != ".bmp" {
			r.Warnings = append(r.Warnings, "Unsupported embedded media retained only in the original file: "+filepath.Base(f.Name))
			continue
		}
		count++
		if count > 100 {
			return domain.Invalid("Document exceeds 100 embedded images")
		}
		rc, e := f.Open()
		if e != nil {
			return e
		}
		b, e := io.ReadAll(io.LimitReader(rc, domain.MaxBytes+1))
		rc.Close()
		if e != nil || len(b) > domain.MaxBytes {
			return domain.Invalid("Embedded image exceeds reading limits")
		}
		path := filepath.Join(dir, fmt.Sprintf("image-%d%s", count, ext))
		if e = os.WriteFile(path, b, 0600); e != nil {
			return e
		}
		segment, e := ocr(ctx, path, dir)
		if e != nil {
			r.Warnings = append(r.Warnings, "An embedded image could not be read; review the original file.")
			continue
		}
		if strings.TrimSpace(segment.Text) != "" {
			segment.Kind = "embedded_image"
			segment.Label = f.Name
			r.Segments = append(r.Segments, segment)
			r.Warnings = append(r.Warnings, "Embedded-image OCR is listed separately; its position in the document is not inferred.")
		}
	}
	return nil
}

// Every container part is accounted for. Metadata is distinct from content that
// this reader cannot interpret; the latter remains an explicit review gap.
func inventoryOfficeParts(data []byte, format string, r *domain.Result) {
	z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		return
	}
	for _, f := range z.File {
		p := f.Name
		state, reason := "metadata", "Container/style/relationship metadata; retained in original"
		if f.FileInfo().IsDir() {
			state, reason = "directory", "Container directory"
		} else if format == "docx" && (p == "word/document.xml" || strings.HasPrefix(p, "word/header") || strings.HasPrefix(p, "word/footer") || p == "word/comments.xml" || p == "word/footnotes.xml" || p == "word/endnotes.xml") {
			state, reason = "read", "Text extracted with source-part locator"
		} else if format == "xlsx" && (strings.HasPrefix(p, "xl/worksheets/sheet") || p == "xl/sharedStrings.xml" || p == "xl/workbook.xml") {
			state, reason = "read", "Workbook sheets/cells enumerated; cached values, formulas never executed"
		} else if strings.Contains(p, "/media/") {
			state, reason = "image_review", "Embedded image OCR attempted separately; inspect result warnings and original"
		} else if strings.Contains(p, "/charts/") || strings.Contains(p, "/diagrams/") || strings.Contains(p, "/drawings/") || strings.Contains(p, "/comments") || strings.Contains(p, "/pivot") || strings.Contains(p, "/customXml/") || strings.HasPrefix(p, "customXml/") {
			state, reason = "uninterpreted", "Content not interpreted; manual original-source review required"
			r.Warnings = append(r.Warnings, "Uninterpreted Office part: "+p+"; review the original")
		}
		r.Parts = append(r.Parts, domain.PartCoverage{Path: p, Disposition: state, Reason: reason})
	}
}
