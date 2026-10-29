package domain

import (
	"fmt"
	"strings"
)

// EvidenceMarkdown includes the lower-priority material in an explicit appendix.
// It is derived, unapproved evidence, not a replacement for original documents.
func EvidenceMarkdown(in TSCInput) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, "# TSC source evidence\n\nDerived, unapproved reading. Source revisions remain authoritative.\n\nEngine: %s\nDocuments: %d; segments: %d; empty segments: %d; low-confidence segments: %d; blocks: %d; repeated furniture: %d.\n\n", in.Coverage.Version, in.Coverage.Documents, in.Coverage.Segments, in.Coverage.EmptySegments, in.Coverage.LowConfidence, in.Coverage.Blocks, in.Coverage.Suppressed)
	for _, suppressed := range []bool{false, true} {
		if suppressed {
			out.WriteString("# Repeated page furniture retained for fallback\n\n")
		}
		for _, p := range in.Passages {
			if p.Suppressed != suppressed {
				continue
			}
			fmt.Fprintf(&out, "## %s / %s\n\nSource: %s; revision: %d; SHA256: %s; method: %s; type: %s\n\n", p.Filename, p.Locator, p.ID, p.Revision, p.SHA256, p.Method, p.Kind)
			out.WriteString(p.Text)
			out.WriteString("\n\n")
		}
	}
	return []byte(out.String())
}
