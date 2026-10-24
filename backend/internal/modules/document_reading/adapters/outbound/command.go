package outbound

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"time"
)

type limitedOutput struct {
	bytes.Buffer
	max int
}

func (w *limitedOutput) Write(b []byte) (int, error) {
	if len(b) > w.max-w.Len() {
		return 0, errors.New("reader output limit")
	}
	return w.Buffer.Write(b)
}
func command(ctx context.Context, limit int, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/tmp", "LANG=C.UTF-8", "OMP_THREAD_LIMIT=1", "MAGICK_MEMORY_LIMIT=256MiB", "MAGICK_MAP_LIMIT=256MiB", "MAGICK_DISK_LIMIT=256MiB", "MAGICK_THREAD_LIMIT=1"}
	out := &limitedOutput{max: limit}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 2 * time.Second
	if e := cmd.Run(); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
