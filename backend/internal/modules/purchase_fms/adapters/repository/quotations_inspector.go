package repository

import (
	"archive/zip"
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

// Inspector checks declared type against content before evidence is encrypted.
// Spreadsheet containers are expanded under fixed limits. Files remain evidence;
// this adapter does not execute workbook formulas or external relationships.
type QuotationsInspector struct{}

func (QuotationsInspector) Inspect(name, mime string, data []byte) error {
	invalid := func() error {
		return domain.QuotationsInvalid("file", "Upload a valid PDF, PNG, JPEG or XLSX quotation file matching its filename and content type")
	}
	if len(data) == 0 || len(data) > 10<<20 || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 || name == "" || strings.ContainsAny(name, "/\\\x00\r\n") {
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
	case ".png":
		if mime != "image/png" || !bytes.HasPrefix(data, []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
			return invalid()
		}
	case ".jpg", ".jpeg":
		if mime != "image/jpeg" || !bytes.HasPrefix(data, []byte{255, 216, 255}) || !bytes.HasSuffix(data, []byte{255, 217}) {
			return invalid()
		}
	case ".xlsx":
		if mime != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			return invalid()
		}
		z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if e != nil || len(z.File) > 2000 {
			return invalid()
		}
		seen := map[string]bool{}
		var expanded uint64
		for _, f := range z.File {
			if seen[f.Name] || strings.Contains(f.Name, "\\") || strings.HasPrefix(f.Name, "/") || strings.Contains("/"+f.Name+"/", "/../") || strings.Contains(strings.ToLower(f.Name), "vbaproject") {
				return invalid()
			}
			seen[f.Name] = true
			if f.UncompressedSize64 > 32<<20 || expanded > 64<<20-f.UncompressedSize64 {
				return invalid()
			}
			expanded += f.UncompressedSize64
			r, e := f.Open()
			if e != nil {
				return invalid()
			}
			n, e := io.Copy(io.Discard, io.LimitReader(r, int64(f.UncompressedSize64)+1))
			closeErr := r.Close()
			if e != nil || closeErr != nil || uint64(n) != f.UncompressedSize64 {
				return invalid()
			}
		}
		if !seen["[Content_Types].xml"] || !seen["xl/workbook.xml"] {
			return invalid()
		}
	default:
		return invalid()
	}
	if mime == "image/png" || mime == "image/jpeg" {
		config, format, e := image.DecodeConfig(bytes.NewReader(data))
		if e != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width) > 40000000/int64(config.Height) || (mime == "image/png" && format != "png") || (mime == "image/jpeg" && format != "jpeg") {
			return invalid()
		}
	}
	return nil
}
