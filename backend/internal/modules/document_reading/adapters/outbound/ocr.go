package outbound

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func readPDF(ctx context.Context, path, dir string, r *domain.Result) error {
	info, e := command(ctx, 1<<20, "pdfinfo", "-box", path)
	if e != nil {
		return domain.Invalid("PDF could not be opened; check damage or password protection")
	}
	match := regexp.MustCompile(`(?m)^Pages:\s+(\d+)`).FindSubmatch(info)
	if len(match) != 2 {
		return domain.Invalid("PDF page count is unavailable")
	}
	pages, _ := strconv.Atoi(string(match[1]))
	if pages < 1 || pages > 1000 {
		return domain.Invalid("PDF must contain 1–1,000 pages")
	}
	for page := 1; page <= pages; page++ {
		if e = ctx.Err(); e != nil {
			return e
		}
		n := strconv.Itoa(page)
		raw, e := command(ctx, 8<<20, "pdftotext", "-f", n, "-l", n, "-layout", "-enc", "UTF-8", path, "-")
		if e != nil {
			return domain.Invalid("PDF text could not be read")
		}
		text := strings.TrimSpace(string(raw))
		segment := domain.Segment{Kind: "page", Label: fmt.Sprintf("Page %d", page), Text: text, Method: "native"}
		garbled := 0
		for _, v := range text {
			if v == unicode.ReplacementChar || v == 0 {
				garbled++
			}
		}
		needsOCR := len([]rune(text)) < 40 || garbled > 3
		images, err := command(ctx, 1<<20, "pdfimages", "-f", n, "-l", n, "-list", path)
		if err == nil {
			for _, line := range strings.Split(string(images), "\n") {
				cols := strings.Fields(line)
				if len(cols) >= 14 && cols[2] == "image" {
					width, _ := strconv.ParseFloat(cols[3], 64)
					height, _ := strconv.ParseFloat(cols[4], 64)
					xdpi, _ := strconv.ParseFloat(cols[12], 64)
					ydpi, _ := strconv.ParseFloat(cols[13], 64)
					if xdpi > 0 && ydpi > 0 && width*height/(xdpi*ydpi) > 35 {
						needsOCR = true
					}
				}
			}
		}
		if needsOCR {
			output := filepath.Join(dir, "page-"+n)
			if _, e = command(ctx, 1<<20, "pdftoppm", "-f", n, "-l", n, "-singlefile", "-r", "300", "-scale-to", "4000", "-png", path, output); e != nil {
				return domain.Invalid("PDF page rendering failed or exceeded limits")
			}
			segment, e = ocr(ctx, output+".png", dir)
			os.Remove(output + ".png")
			if e != nil {
				return domain.Invalid("A scanned PDF page could not be read; inspect scan quality or split the source")
			}
			segment.Kind = "page"
			segment.Label = fmt.Sprintf("Page %d", page)
			segment.NativeText = text
			if strings.TrimSpace(segment.Text) == "" && text != "" {
				segment.Text = text
				segment.Method = "native_ocr_empty"
				r.Warnings = append(r.Warnings, segment.Label+": OCR returned no text; native text retained.")
			}
			if segment.Confidence != nil && *segment.Confidence < 70 {
				r.Warnings = append(r.Warnings, segment.Label+": low OCR confidence; compare with the original before using figures or dates.")
			}
			if domain.PreferNativeText(text, segment.Text, segment.Confidence) {
				segment.OCRText = segment.Text
				segment.OCRConfidence = segment.Confidence
				segment.Text = text
				segment.Method = "native_with_ocr_alternative"
				segment.Confidence = nil
				segment.Words = nil
				segment.CoordinateUnit = ""
				if bbox, err := command(ctx, 8<<20, "pdftotext", "-f", n, "-l", n, "-bbox-layout", path, "-"); err == nil {
					segment.Words = pdfWords(bbox)
					segment.CoordinateUnit = "pdf_points"
				}
				r.Warnings = append(r.Warnings, segment.Label+": clean native text retained as primary; low-confidence OCR preserved as an alternative.")
			}
		} else {
			bbox, e := command(ctx, 8<<20, "pdftotext", "-f", n, "-l", n, "-bbox-layout", path, "-")
			if e == nil {
				segment.Words = pdfWords(bbox)
				segment.CoordinateUnit = "pdf_points"
			}
		}
		if segment.Text == "" {
			r.Warnings = append(r.Warnings, segment.Label+": no text detected; page may be blank or unreadable.")
		}
		r.Segments = append(r.Segments, segment)
	}
	return nil
}
func pdfWords(b []byte) []domain.Word {
	d := xml.NewDecoder(bytes.NewReader(b))
	out := []domain.Word{}
	for {
		token, e := d.Token()
		if e != nil {
			break
		}
		if s, ok := token.(xml.StartElement); ok && s.Name.Local == "word" {
			attrs := map[string]float64{}
			for _, a := range s.Attr {
				attrs[a.Name.Local], _ = strconv.ParseFloat(a.Value, 64)
			}
			var text string
			if e = d.DecodeElement(&text, &s); e == nil {
				out = append(out, domain.Word{Text: text, Confidence: 100, X: attrs["xMin"], Y: attrs["yMin"], Width: attrs["xMax"] - attrs["xMin"], Height: attrs["yMax"] - attrs["yMin"]})
			}
		}
	}
	return out
}
func ocr(ctx context.Context, path, dir string) (domain.Segment, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	segment := domain.Segment{Method: "ocr", CoordinateUnit: "pixels"}
	dimensions, e := command(ctx, 4096, "identify", "-ping", "-format", "%w %h", path)
	if e != nil {
		return segment, e
	}
	size := strings.Fields(string(dimensions))
	if len(size) != 2 {
		return segment, domain.Invalid("Image dimensions could not be determined")
	}
	w, _ := strconv.ParseInt(size[0], 10, 64)
	h, _ := strconv.ParseInt(size[1], 10, 64)
	if w < 1 || h < 1 || w > 16000000/h {
		return segment, domain.Invalid("Scan exceeds the 16-million-pixel limit")
	}
	prepared := filepath.Join(dir, "prepared.png")
	if _, e = command(ctx, 1<<20, "convert", path, "-colorspace", "Gray", "-background", "white", "-alpha", "remove", "-deskew", "40%", prepared); e != nil {
		return segment, e
	}
	defer os.Remove(prepared)
	osd, e := command(ctx, 64<<10, "tesseract", prepared, "stdout", "--psm", "0", "-l", "osd")
	if e == nil {
		m := regexp.MustCompile(`Rotate: (90|180|270)`).FindSubmatch(osd)
		if len(m) == 2 {
			if _, e = command(ctx, 1<<20, "convert", prepared, "-rotate", string(m[1]), prepared); e != nil {
				return segment, e
			}
		}
	}
	tsv, e := command(ctx, 8<<20, "tesseract", prepared, "stdout", "-l", "eng+hin", "--psm", "3", "tsv")
	if e != nil {
		return segment, e
	}
	scan := bufio.NewScanner(bytes.NewReader(tsv))
	scan.Buffer(make([]byte, 4096), 1<<20)
	sum := 0.0
	lastLine := ""
	var text strings.Builder
	for scan.Scan() {
		c := strings.SplitN(scan.Text(), "\t", 12)
		if len(c) != 12 || c[0] != "5" {
			continue
		}
		confidence, _ := strconv.ParseFloat(c[10], 64)
		if confidence < 0 || strings.TrimSpace(c[11]) == "" {
			continue
		}
		line := strings.Join(c[1:5], ":")
		if lastLine != "" {
			if line == lastLine {
				text.WriteString(" ")
			} else {
				text.WriteString("\n")
			}
		}
		lastLine = line
		text.WriteString(c[11])
		x, _ := strconv.ParseFloat(c[6], 64)
		y, _ := strconv.ParseFloat(c[7], 64)
		w, _ := strconv.ParseFloat(c[8], 64)
		h, _ := strconv.ParseFloat(c[9], 64)
		segment.Words = append(segment.Words, domain.Word{Text: c[11], Confidence: confidence, X: x, Y: y, Width: w, Height: h})
		sum += confidence
	}
	if e = scan.Err(); e != nil {
		return segment, e
	}
	segment.Text = text.String()
	if len(segment.Words) > 0 {
		v := sum / float64(len(segment.Words))
		segment.Confidence = &v
	}
	return segment, nil
}
