// Package pdfreport renders business-neutral printable tables; callers own content and authorization.
package pdfreport

import (
	"context"
	"fmt"
	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"math"
	"strings"
)

type Column struct {
	Title string
	Width float64
}
type Color struct{ R, G, B uint8 }
type Fill struct{ Colors []Color }
type LegendEntry struct {
	Label string
	Fill  Fill
}
type Document struct {
	ColumnFills            map[int]Fill
	RowFills               map[int]Fill
	Legend                 []LegendEntry
	Title, Subtitle, Stamp string
	Columns                []Column
	Rows                   [][]string
	Notes                  []string
}

func Render(ctx context.Context, d Document) ([]byte, error) {
	const width = 841.89
	const height = 595.28
	const margin = 24.
	const line = 13.
	const bottom = height - 44
	if len(d.Columns) == 0 || len(d.Columns) > 16 || len(d.Rows) > 10000 {
		return nil, fmt.Errorf("invalid PDF table bounds")
	}
	sum := 0.
	for _, c := range d.Columns {
		if c.Width < 20 {
			return nil, fmt.Errorf("invalid column width")
		}
		sum += c.Width
	}
	if sum > width-2*margin+.1 {
		return nil, fmt.Errorf("columns exceed landscape page")
	}
	p := gopdf.GoPdf{}
	p.Start(gopdf.Config{PageSize: gopdf.Rect{W: width, H: height}})
	if e := p.AddTTFFontData("body", goregular.TTF); e != nil {
		return nil, e
	}
	if e := p.AddTTFFontData("bold", gobold.TTF); e != nil {
		return nil, e
	}
	if e := p.SetFont("body", "", 10); e != nil {
		return nil, e
	}
	fill := func(x, y, w, h float64, f Fill) {
		if len(f.Colors) == 0 {
			return
		}
		for i, c := range f.Colors {
			p.SetFillColor(c.R, c.G, c.B)
			p.RectFromUpperLeftWithStyle(x+float64(i)*w/float64(len(f.Colors)), y, w/float64(len(f.Colors)), h, "F")
		}
	}
	y := 0.
	page := 0
	text := func(x, y float64, s string, bold bool) error {
		if s == "" {
			return nil
		}
		font := "body"
		size := 10
		if bold {
			font = "bold"
			size = 11
		}
		if e := p.SetFont(font, "", size); e != nil {
			return e
		}
		p.SetXY(x, y)
		return p.CellWithOption(&gopdf.Rect{W: width - x - margin, H: line}, s, gopdf.CellOption{})
	}
	newPage := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		page++
		p.AddPage()
		p.SetTextColor(23, 44, 66)
		p.SetXY(margin, margin)
		for _, s := range []string{d.Title, d.Subtitle} {
			p.SetFont("bold", "", 11)
			parts, e := p.SplitTextWithOption(s, width-2*margin, &gopdf.BreakOption{Mode: gopdf.BreakModeIndicatorSensitive, BreakIndicator: ' '})
			if e != nil {
				return e
			}
			for _, t := range parts {
				if e = text(margin, p.GetY(), t, true); e != nil {
					return e
				}
				p.SetY(p.GetY() + line)
			}
		}

		y = p.GetY() + 10
		legendX := margin
		for _, entry := range d.Legend {
			if e := p.SetFont("body", "", 10); e != nil {
				return e
			}
			textWidth, e := p.MeasureTextWidth(entry.Label)
			if e != nil {
				return e
			}
			if legendX+textWidth+40 > width-margin {
				legendX = margin
				y += line + 4
			}
			fill(legendX, y+2, 16, 9, entry.Fill)
			if e := text(legendX+22, y, entry.Label, false); e != nil {
				return e
			}
			legendX += textWidth + 42
		}
		if len(d.Legend) > 0 {
			y += line + 10
		}
		headerLines := make([][]string, len(d.Columns))
		maxLines := 1
		if e := p.SetFont("bold", "", 11); e != nil {
			return e
		}
		for i, c := range d.Columns {
			parts, e := p.SplitTextWithOption(c.Title, c.Width-8, &gopdf.BreakOption{Mode: gopdf.BreakModeIndicatorSensitive, BreakIndicator: ' '})
			if e != nil {
				return e
			}
			headerLines[i] = parts
			if len(parts) > maxLines {
				maxLines = len(parts)
			}
		}
		headerHeight := math.Max(26, float64(maxLines)*line+10)
		if y+headerHeight > bottom-26 {
			return fmt.Errorf("PDF column headings exceed the page")
		}
		p.SetFillColor(227, 235, 246)
		p.RectFromUpperLeftWithStyle(margin, y, sum, headerHeight, "F")
		x := margin
		for i, c := range d.Columns {
			fill(x, y, c.Width, headerHeight, d.ColumnFills[i])
			for n, t := range headerLines[i] {
				if e := text(x+4, y+5+float64(n)*line, t, true); e != nil {
					return e
				}
			}
			x += c.Width
		}
		y += headerHeight

		p.SetStrokeColor(206, 215, 226)
		p.Line(margin, height-34, width-margin, height-34)
		if e := text(margin, height-28, d.Stamp, false); e != nil {
			return e
		}
		return text(width-90, height-28, fmt.Sprintf("Page %d", page), false)
	}
	if e := newPage(); e != nil {
		return nil, e
	}
	for ri, row := range d.Rows {
		if len(row) != len(d.Columns) {
			return nil, fmt.Errorf("invalid PDF row")
		}
		p.SetFont("body", "", 10)
		cells := make([][]string, len(row))
		total := 1
		for i, v := range row {
			v = strings.ReplaceAll(v, "\r", "")
			for _, part := range strings.Split(v, "\n") {
				if part == "" {
					cells[i] = append(cells[i], "")
					continue
				}
				parts, e := p.SplitTextWithOption(part, d.Columns[i].Width-8, &gopdf.BreakOption{Mode: gopdf.BreakModeIndicatorSensitive, BreakIndicator: ' '})
				if e != nil {
					return nil, e
				}
				if len(parts) == 0 {
					parts = []string{""}
				}
				cells[i] = append(cells[i], parts...)
			}
			total = max(total, len(cells[i]))
		}
		for offset := 0; offset < total; {
			available := int(math.Floor((bottom - y - 8) / line))
			if available < 1 {
				if e := newPage(); e != nil {
					return nil, e
				}
				continue
			}
			count := min(total-offset, available)
			h := float64(count)*line + 8
			if ri%2 == 1 {
				p.SetFillColor(245, 248, 252)
				p.RectFromUpperLeftWithStyle(margin, y, sum, h, "F")
			}
			fill(margin, y, sum, h, d.RowFills[ri])
			x := margin
			p.SetStrokeColor(210, 220, 232)
			for i, c := range d.Columns {
				fill(x, y, c.Width, h, d.ColumnFills[i])
				p.RectFromUpperLeftWithStyle(x, y, c.Width, h, "D")
				for n := 0; n < count; n++ {
					if offset+n < len(cells[i]) {
						if e := text(x+4, y+4+float64(n)*line, cells[i][offset+n], false); e != nil {
							return nil, e
						}
					}
				}
				x += c.Width
			}
			y += h
			offset += count
		}
	}
	for _, note := range d.Notes {
		p.SetFont("body", "", 10)
		parts, e := p.SplitTextWithOption(note, sum, &gopdf.BreakOption{Mode: gopdf.BreakModeIndicatorSensitive, BreakIndicator: ' '})
		if e != nil {
			return nil, e
		}
		y += 10
		for _, s := range parts {
			if y+line > bottom {
				if e := newPage(); e != nil {
					return nil, e
				}
			}
			if e := text(margin, y, s, false); e != nil {
				return nil, e
			}
			y += line
		}
	}
	return p.GetBytesPdf(), nil
}
