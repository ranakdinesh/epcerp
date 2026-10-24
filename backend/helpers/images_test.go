package helpers

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestCanonicalImageBoundsAndContent(t *testing.T) {
	var raw bytes.Buffer
	if err := png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 40, 20))); err != nil {
		t.Fatal(err)
	}
	canonical, err := (ImageProcessor{MaximumBytes: 512 * 1024, MaximumDimension: 2048, Size: 256}).Canonical(raw.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(canonical))
	if err != nil || format != "jpeg" || cfg.Width != 256 || cfg.Height != 256 {
		t.Fatal("image not canonical", err)
	}
	for _, input := range [][]byte{nil, []byte("<svg onload='bad'></svg>"), make([]byte, 512*1024+1)} {
		if _, err = (ImageProcessor{MaximumBytes: 512 * 1024, MaximumDimension: 2048, Size: 256}).Canonical(input); err == nil {
			t.Fatal("unsafe upload accepted")
		}
	}
	raw.Reset()
	_ = png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 2049, 1)))
	if _, err = (ImageProcessor{MaximumBytes: 512 * 1024, MaximumDimension: 2048, Size: 256}).Canonical(raw.Bytes()); err == nil {
		t.Fatal("pixel bound ignored")
	}
}
