package repository

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Inspector validates passive PDF/image evidence before storage. It does not
// execute PDF actions or treat document contents as workflow approval.
type TechnicalInspector struct{}

func (TechnicalInspector) Inspect(name, mime string, data []byte) error {
	invalid := func() error {
		return domain.TechnicalInvalid("file", "Upload a valid PDF, PNG or JPEG matching its filename and content type")
	}
	if len(data) == 0 || len(data) > domain.TechnicalMaxFileBytes || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 || strings.TrimSpace(name) == "" || strings.ContainsAny(name, "/\\") {
		return invalid()
	}
	for _, r := range name {
		if r < 32 || r == 127 {
			return invalid()
		}
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		if mime != "application/pdf" || !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data[max(0, len(data)-2048):], []byte("%%EOF")) {
			return invalid()
		}
		return nil
	case ".png":
		if mime != "image/png" || !bytes.HasPrefix(data, []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
			return invalid()
		}
	case ".jpg", ".jpeg":
		if mime != "image/jpeg" || !bytes.HasPrefix(data, []byte{255, 216, 255}) || !bytes.HasSuffix(data, []byte{255, 217}) {
			return invalid()
		}
	default:
		return invalid()
	}
	config, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width) > 40000000/int64(config.Height) || mime == "image/png" && format != "png" || mime == "image/jpeg" && format != "jpeg" {
		return invalid()
	}
	if _, _, e = image.Decode(bytes.NewReader(data)); e != nil {
		return invalid()
	}
	return nil
}
