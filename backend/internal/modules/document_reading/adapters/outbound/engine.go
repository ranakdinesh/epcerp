package outbound

import (
	"context"
	"encoding/json"
	"github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ProcessEngine gives untrusted parsers a minimal environment, bounded output and
// separate process tree. Database credentials and encryption keys never enter it.
type ProcessEngine struct{ Binary string }

func (e ProcessEngine) Read(ctx context.Context, data []byte, format, hash string) (domain.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	dir, err := os.MkdirTemp("", "epcerp-source-")
	if err != nil {
		return domain.Result{}, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "source."+format)
	if err = os.WriteFile(path, data, 0600); err != nil {
		return domain.Result{}, err
	}
	cmd := exec.CommandContext(ctx, "prlimit", "--as=1610612736", "--cpu=900", "--nofile=128", "--", e.Binary, "-input", path, "-format", format, "-sha256", hash)
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/tmp", "LANG=C.UTF-8", "OMP_THREAD_LIMIT=1"}
	out := &limitedOutput{max: 8 << 20}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	configureProcess(cmd)
	if err = cmd.Run(); err != nil {
		return domain.Result{}, domain.Invalid("Document could not be read; check scan quality, password protection or processing limits")
	}
	var r domain.Result
	if err = json.Unmarshal(out.Bytes(), &r); err != nil {
		return r, err
	}
	if r.SourceSHA256 != hash {
		return r, domain.Invalid("Reader result does not match the original document")
	}
	return r, domain.CheckResult(r)
}
