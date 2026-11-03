package domain

import (
	"strings"
)

const TSCOriginalAllowancePaise int64 = 500
const TSCOriginalModelVersion = "tsc-original-pdf-v1"

type TSCOriginal struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
	Pages    int32  `json:"pages"`
	Bytes    int    `json:"bytes"`
	Data     []byte `json:"-"`
}
type TSCResearchOptions struct{ OriginalDocumentsConfirmed bool }

func OriginalForTSC(s DraftSource, data []byte) (TSCOriginal, error) {
	o := TSCOriginal{ID: s.ID, Filename: s.Path, SHA256: s.SHA256, Pages: s.ExpectedPages, Bytes: len(data), Data: data}
	if o.Pages == 0 {
		o.Pages = s.Segments
	}

	if !strings.HasSuffix(strings.ToLower(s.Path), ".pdf") || (len(data) < 5 || string(data[:5]) != "%PDF-") || len(s.SHA256) != 64 || o.Pages < 1 {
		return o, Invalid("Original-document review requires verified PDF versions of every source attachment; no unsupported source is silently skipped")
	}
	return o, nil
}
func ValidateTSCOriginals(docs []TSCOriginal) error {
	size, pages := 0, int32(0)
	seen := map[string]bool{}
	if len(docs) == 0 || len(docs) > 8 {
		return Invalid("Original-document review supports 1–8 source PDFs per request")
	}
	for _, d := range docs {
		if d.Bytes != len(d.Data) || d.Bytes < 1 || d.Bytes > 8<<20 || d.Pages < 1 || d.Pages > 50 {
			return Invalid("Original source metadata exceeds review limits")
		}
		if seen[d.ID] {
			return Invalid("Duplicate original source")
		}
		seen[d.ID] = true
		size += d.Bytes
		pages += d.Pages
	}
	if size > 8<<20 || pages > 50 {
		return Invalid("The complete source package exceeds the original-review limit (8 MiB or 50 pages); no attachments were sent")
	}
	return nil
}
