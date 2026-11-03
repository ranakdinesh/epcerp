package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type TableCell struct {
	Row          int     `json:"row"`
	Column       int     `json:"column"`
	RowSpan      int     `json:"row_span"`
	ColumnSpan   int     `json:"column_span"`
	Text         string  `json:"text"`
	Header       bool    `json:"header"`
	ColumnHeader string  `json:"column_header,omitempty"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Width        float64 `json:"width"`
	Height       float64 `json:"height"`
}
type PDFTable struct {
	ID    string      `json:"id"`
	Cells []TableCell `json:"cells"`
}
type PageTables struct {
	Page   int        `json:"page"`
	Width  float64    `json:"width"`
	Height float64    `json:"height"`
	Tables []PDFTable `json:"tables"`
}
type PDFTables struct {
	Version string       `json:"version"`
	Pages   []PageTables `json:"pages"`
}

func (t PDFTables) Validate() error {
	if t.Version != "pdf-tables-v1" || len(t.Pages) > 100 {
		return Invalid("Invalid table coverage")
	}
	count := 0
	for i, p := range t.Pages {
		if p.Page != i+1 || p.Width <= 0 || p.Height <= 0 || p.Width > 20000 || p.Height > 20000 || math.IsNaN(p.Width) || math.IsNaN(p.Height) || math.IsInf(p.Width, 0) || math.IsInf(p.Height, 0) {
			return Invalid("Invalid table page")
		}
		seen := map[string]bool{}
		for _, table := range p.Tables {
			if table.ID == "" || seen[table.ID] {
				return Invalid("Invalid table identity")
			}
			seen[table.ID] = true
			for _, c := range table.Cells {
				count++
				if count > 12000 || c.Row < 0 || c.Column < 0 || c.RowSpan < 1 || c.ColumnSpan < 1 || len(c.Text) > 12000 || c.X < 0 || c.Y < 0 || c.Width <= 0 || c.Height <= 0 || c.X+c.Width > p.Width+1 || c.Y+c.Height > p.Height+1 {
					return Invalid("Invalid table cell")
				}
				for _, v := range []float64{c.X, c.Y, c.Width, c.Height} {
					if math.IsNaN(v) || math.IsInf(v, 0) {
						return Invalid("Invalid table coordinates")
					}
				}
			}
		}
	}
	return nil
}

// Row cells stay atomic, including wrapped/vertically centred values and spans.
// Original page text remains intact; table evidence is a derived representation.
func tableEvidence(segment int, s Segment) []EvidenceBlock {
	var out []EvidenceBlock
	for _, t := range s.Tables {
		groups := map[int][]TableCell{}
		for _, c := range t.Cells {
			groups[c.Row] = append(groups[c.Row], c)
		}
		keys := []int{}
		for row := range groups {
			keys = append(keys, row)
		}
		sort.Ints(keys)
		for _, row := range keys {
			cells := groups[row]
			sort.Slice(cells, func(i, j int) bool { return cells[i].Column < cells[j].Column })
			var text strings.Builder
			for _, c := range cells {
				if c.ColumnHeader != "" {
					fmt.Fprintf(&text, "[%s] ", c.ColumnHeader)
				}
				fmt.Fprintf(&text, "%s | ", strings.TrimSpace(c.Text))
			}
			raw := strings.TrimSuffix(text.String(), " | ")
			if strings.TrimSpace(raw) == "" {
				continue
			}
			out = append(out, EvidenceBlock{ID: fmt.Sprintf("%d:table:%s:row:%d", segment+1, t.ID, row), Segment: segment, Label: fmt.Sprintf("%s / table %s row %d (PDF coordinates retained)", s.Label, t.ID, row+1), Kind: "table_row", Text: raw, SearchText: SearchText(raw), Method: "native_table", TableCells: cells})
		}
	}
	return out
}

func tableTextOverlap(s Segment, raw string) bool {
	q := SearchText(raw)
	if len(q) < 5 {
		return false
	}
	for _, t := range s.Tables {
		for _, c := range t.Cells {
			v := SearchText(c.Text)
			if len(v) >= 5 && (strings.Contains(q, v) || (len(q) > 20 && strings.Contains(v, q))) {
				return true
			}
		}
	}
	return false
}
