package infrastructure

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"testing"
)

func TestDocumentInspectorRejectsRenamedExecutableAndBrokenImages(t *testing.T) {
	for _, data := range [][]byte{[]byte("<html>document.pdf</html>"), []byte("<svg></svg>"), []byte("%PDF-1.7 incomplete"), {0x89, 'P', 'N', 'G', 13, 10, 26, 10}} {
		if _, e := NewDocumentInspector(15 * 1024 * 1024).Inspect(data); e == nil {
			t.Fatal("invalid document accepted")
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	kind, e := NewDocumentInspector(15 * 1024 * 1024).Inspect(b.Bytes())
	if e != nil || kind != "image/png" {
		t.Fatal("valid image rejected", e)
	}
}

func TestDocumentInspectorUsesCallerSizeLimit(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	var failure *DocumentValidationError
	if _, err := NewDocumentInspector(data.Len() - 1).Inspect(data.Bytes()); !errors.As(err, &failure) {
		t.Fatal("caller size limit was not enforced", err)
	}
	if failure.ValidationMessage() != failure.Error() {
		t.Fatal("unsafe validation message")
	}
	if _, err := NewDocumentInspector(data.Len()).Inspect(data.Bytes()); err != nil {
		t.Fatal(err)
	}
}
