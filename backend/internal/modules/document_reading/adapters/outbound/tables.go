package outbound

import (
	"context"
	"encoding/json"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	"os"
	"path/filepath"
	"time"
)

type PDFLayouts struct{}

func (PDFLayouts) Extract(ctx context.Context, data []byte) (out domain.PDFTables, err error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if len(data) > domain.MaxBytes {
		return out, domain.Invalid("PDF table input limit")
	}
	dir, err := os.MkdirTemp("", "epcerp-tables-")
	if err != nil {
		return out, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "input.pdf")
	if err = os.WriteFile(path, data, 0600); err != nil {
		return out, err
	}
	raw, err := command(ctx, 8<<20, "prlimit", "--as=1073741824", "--cpu=20", "--nofile=128", "--", "/opt/pdf-tables/bin/python", "/app/pdf_tables.py", "--input", path)
	if err != nil {
		return out, domain.Invalid("Table layout unavailable; inspect originals")
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return out, domain.Invalid("Invalid table geometry")
	}
	return out, out.Validate()
}
