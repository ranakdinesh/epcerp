package repository

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestTechnicalPassiveTechnicalEvidenceTypeSizeAndContent(t *testing.T) {
	var b bytes.Buffer
	if e := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); e != nil {
		t.Fatal(e)
	}
	valid := b.Bytes()
	pdf := []byte("%PDF-1.7\nActual passive test fixture\n%%EOF")
	for _, q := range []struct {
		name, mime string
		data       []byte
		valid      bool
	}{
		{"GA.pdf", "application/pdf", pdf, true},
		{"Scan.PNG", "image/png", valid, true},
		{"../GA.pdf", "application/pdf", pdf, false},
		{"bad\n.pdf", "application/pdf", pdf, false},
		{"GA.pdf", "image/png", pdf, false},
		{"GA.pdf", "application/pdf", []byte("<html>fake</html>"), false},
		{"scan.png", "image/png", valid[:len(valid)-4], false},
		{"workbook.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", pdf, false},
		{"empty.pdf", "application/pdf", nil, false},
		{"large.pdf", "application/pdf", make([]byte, (10<<20)+1), false},
	} {
		if e := (TechnicalInspector{}).Inspect(q.name, q.mime, q.data); (e == nil) != q.valid {
			t.Fatalf("%q acceptance=%v: %v", q.name, q.valid, e)
		}
	}
}
