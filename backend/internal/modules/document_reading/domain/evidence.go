package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const EvidenceVersion = "document-evidence-v1"

// Offsets address the unchanged Text (or NativeText) of the source segment.
type EvidenceBlock struct {
	ID          string   `json:"id"`
	Segment     int      `json:"segment"`
	Label       string   `json:"label"`
	Kind        string   `json:"kind"`
	Heading     string   `json:"heading,omitempty"`
	Start       int      `json:"start"`
	End         int      `json:"end"`
	Text        string   `json:"text"`
	SearchText  string   `json:"search_text"`
	Method      string   `json:"method"`
	Confidence  *float64 `json:"confidence,omitempty"`
	Alternative bool     `json:"alternative,omitempty"`
	Suppressed  bool     `json:"suppressed,omitempty"`
	Reason      string   `json:"reason,omitempty"`
}
type Evidence struct {
	Version       string          `json:"version"`
	Segments      int             `json:"segments"`
	EmptySegments int             `json:"empty_segments"`
	Suppressed    int             `json:"suppressed"`
	LowConfidence int             `json:"low_confidence"`
	Blocks        []EvidenceBlock `json:"blocks"`
}

var pageLine = regexp.MustCompile(`(?i)^\s*(?:page\s*)?\d+\s*(?:(?:of|/)\s*\d+)?\s*$`)
var clauseLine = regexp.MustCompile(`^\s*(?:\d+(?:\.\d+)*[.)]?|[A-Z][.)])\s+\S`)
var tableGap = regexp.MustCompile(`\S[\t ]{2,}\S`)

func SearchText(s string) string { return strings.Join(strings.Fields(s), " ") }

// BuildEvidence creates a lossless, rebuildable search view. Suppressed material
// stays in Blocks; consumers must retain a fallback path to the full evidence.
func BuildEvidence(r Result) Evidence {
	out := Evidence{Version: EvidenceVersion, Segments: len(r.Segments), Blocks: []EvidenceBlock{}}
	repeated := map[string]int{}
	for _, s := range r.Segments {
		if s.Kind != "page" {
			continue
		}
		lines := strings.Split(s.Text, "\n")
		seen := map[string]bool{}
		for i, line := range lines {
			key := SearchText(line)
			if (i < 2 || i >= len(lines)-2) && len(key) > 3 && !hasDigit(key) && !seen[key] {
				repeated[key]++
				seen[key] = true
			}
		}
	}
	first := map[string]bool{}
	heading := ""
	for si, s := range r.Segments {
		pageMarker := func(text string) bool {
			return pageLine.MatchString(text) && (strings.Contains(strings.ToLower(text), "page") || strings.TrimSpace(text) == fmt.Sprint(si+1))
		}
		if strings.TrimSpace(s.Text) == "" {
			out.EmptySegments++
		}
		if s.Confidence != nil && *s.Confidence < 70 {
			out.LowConfidence++
		}
		texts := []string{s.Text}
		if s.NativeText != "" && SearchText(s.NativeText) != SearchText(s.Text) {
			texts = append(texts, s.NativeText)
		}
		if s.OCRText != "" && SearchText(s.OCRText) != SearchText(s.Text) {
			texts = append(texts, s.OCRText)
		}
		for ti, text := range texts {
			start := 0
			for start < len(text) {
				// Paragraphs/clauses remain intact where possible; very large blocks split
				// on line or sentence boundaries inside the citation byte budget.
				end := len(text)
				if n := strings.Index(text[start:], "\n\n"); n >= 0 {
					end = start + n + 2
				}
				// Keep repeated margin lines separate so suppression never consumes body
				// text. Numbered clauses start a new block without losing their heading.
				offset := start
				for _, line := range strings.SplitAfter(text[start:end], "\n") {
					key := SearchText(line)
					margin := strings.Count(text[:offset], "\n") < 2 || strings.Count(text[offset:], "\n") < 2
					furniture := (pageMarker(key) && margin) || (repeated[key] >= 3 && repeated[key]*2 >= len(r.Segments))
					if offset > start && (furniture || clauseLine.MatchString(key)) {
						end = offset
						break
					}
					offset += len(line)
					if furniture {
						end = offset
						break
					}
				}
				if end-start > 1800 {
					end = start + 1800
					for end > start && !utf8.RuneStart(text[end]) {
						end--
					}
					candidate := text[start:end]
					if n := strings.LastIndex(candidate, "\n"); n > 600 {
						end = start + n + 1
					} else if n := strings.LastIndex(candidate, ". "); n > 600 {
						end = start + n + 2
					}
				}
				raw := text[start:end]
				search := SearchText(raw)
				if search != "" {
					lines := strings.Split(strings.TrimSpace(raw), "\n")
					kind := "paragraph"
					if len(lines) > 1 && tableGap.MatchString(raw) {
						kind = "table"
					}
					if s.Kind == "sheet" {
						kind = "table"
						heading = s.Label
					}
					if len(lines) > 0 && len(lines[0]) < 160 && clauseLine.MatchString(lines[0]) {
						heading = SearchText(lines[0])
					}
					if len(lines) == 1 && len(search) < 160 && (clauseLine.MatchString(search) || strings.ToUpper(search) == search) {
						kind = "heading"
						heading = search
					}
					block := EvidenceBlock{ID: fmt.Sprintf("%d:%d:%d", si+1, ti, start), Segment: si, Label: s.Label, Kind: kind, Heading: heading, Start: start, End: end, Text: raw, SearchText: search, Method: s.Method, Confidence: s.Confidence, Alternative: ti > 0}
					if ti > 0 {
						block.Method = "native_alternative"
						block.Confidence = nil
						if text == s.OCRText {
							block.Method = "ocr_alternative"
							block.Confidence = s.OCRConfidence
						}
					}
					margin := strings.Count(text[:start], "\n") < 2 || strings.Count(text[end:], "\n") < 2
					if s.Kind == "page" && ((pageMarker(search) && margin) || (len(lines) == 1 && repeated[search] >= 3 && repeated[search]*2 >= len(r.Segments) && first[search])) {
						block.Suppressed = true
						block.Reason = "repeated_page_furniture"
						out.Suppressed++
					}
					first[search] = true
					out.Blocks = append(out.Blocks, block)
				}
				start = end
			}
		}
	}
	return out
}

func hasDigit(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// A weak OCR pass must not displace a substantial, clean native text layer.
// Preserve both; this is only a choice of primary search representation.
func PreferNativeText(native, ocr string, confidence *float64) bool {
	if confidence == nil || *confidence >= 65 {
		return false
	}
	clean := 0
	letters := 0
	count := 0
	for _, r := range native {
		if unicode.IsSpace(r) {
			continue
		}
		count++
		if unicode.IsLetter(r) {
			letters++
		}
		if unicode.IsPrint(r) && r != unicode.ReplacementChar {
			clean++
		}
	}
	return count > 100 && letters > 50 && float64(clean)/float64(count) > 0.95 && len(native)*10 >= len(ocr)*7
}
