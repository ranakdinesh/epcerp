package outbound

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
)

// Format previews render the same DOCX bytes offered for download, entirely offline.
type TSCFormatPreviewer struct{}

var formatRenderSlot = make(chan struct{}, 1)

func (TSCFormatPreviewer) Render(ctx context.Context, f domain.TSCFormat, pdf bool) ([]byte, error) {
	in, err := TSCRegistry()
	if err != nil {
		return nil, err
	}
	in.FormatID, in.FormatName, in.FormatVersion, in.Questions = f.ID, f.Name, f.Version, f.Questions
	result := domain.TSCResult{}
	for _, q := range f.Questions {
		result.Answers = append(result.Answers, domain.TSCAnswer{QuestionID: q.ID})
	}
	data, err := (TSCExporter{}).export(domain.TSCJob{Input: in, Result: result}, true)
	if err != nil || !pdf {
		return data, err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	select {
	case formatRenderSlot <- struct{}{}:
		defer func() { <-formatRenderSlot }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	dir, err := os.MkdirTemp("", "tsc-format-preview-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if err = os.WriteFile(filepath.Join(dir, "format.docx"), data, 0600); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "soffice", "-env:UserInstallation=file://"+filepath.Join(dir, "profile"), "--headless", "--nologo", "--nodefault", "--norestore", "--convert-to", "pdf:writer_pdf_Export", "--outdir", dir, filepath.Join(dir, "format.docx"))
	if err = cmd.Run(); err != nil {
		return nil, fmt.Errorf("render TSC format: %w", err)
	}
	data, err = os.ReadFile(filepath.Join(dir, "format.pdf"))
	if err != nil {
		return nil, err
	}
	if len(data) > 16<<20 {
		return nil, fmt.Errorf("format preview exceeds size limit")
	}
	return data, nil
}
