package outbound

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	"strings"
)

type Inspector struct{}

func (i Inspector) Validate(name string, data []byte) (string, string, error) {
	name, format, e := domain.Filename(name)
	if e != nil {
		return "", "", e
	}
	if len(data) == 0 || len(data) > domain.MaxBytes {
		return "", "", domain.Invalid("Document must be at most 32 MiB")
	}
	if e = i.Inspect(data, format); e != nil {
		return "", "", e
	}
	hash := sha256.Sum256(data)
	return name, hex.EncodeToString(hash[:]), nil
}
func (Inspector) Inspect(data []byte, format string) error {
	bad := domain.Invalid("File contents do not match the selected document format, or the document is damaged")
	switch format {
	case "pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return bad
		}
	case "doc", "xls":
		if !bytes.HasPrefix(data, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
			return bad
		}
	case "xlsx", "docx":
		z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if e != nil {
			return bad
		}
		var total uint64
		found := false
		seen := map[string]bool{}
		for _, f := range z.File {
			if seen[f.Name] {
				return bad
			}
			seen[f.Name] = true
			if strings.Contains(f.Name, "..") || strings.HasPrefix(f.Name, "/") {
				return bad
			}
			if f.UncompressedSize64 > 256<<20 || total > 256<<20-f.UncompressedSize64 {
				return domain.Invalid("Document archive exceeds the 256 MiB expansion limit")
			}
			total += f.UncompressedSize64
			if len(z.File) > 10000 {
				return domain.Invalid("Document has too many archive entries")
			}
			if strings.Contains(strings.ToLower(f.Name), "vbaproject") {
				return domain.Invalid("Macro-enabled documents are not supported")
			}
			if format == "xlsx" && f.Name == "xl/workbook.xml" || format == "docx" && f.Name == "word/document.xml" {
				found = true
			}
		}
		if !found {
			return bad
		}
	default:
		return bad
	}
	return nil
}
