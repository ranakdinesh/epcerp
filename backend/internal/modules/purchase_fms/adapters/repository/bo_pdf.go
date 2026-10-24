package repository

import (
	"context"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/pdfreport"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"sort"
	"strings"
	"time"
)

type BOPrinter struct{}

func (BOPrinter) Render(ctx context.Context, v domain.BOPrint) ([]byte, error) {
	rows := [][]string{}
	for _, r := range v.Records {
		for _, l := range r.Lines {
			state := "Draft"
			if r.State == "completed" {
				state = "Finalised"
			}
			val := func(p *string) string {
				if p == nil {
					return ""
				}
				return *p
			}
			name := strings.Split(l.Snapshot.Description, " · ")[0]
			details := l.Reference
			if l.Notes != "" {
				details += "\n" + l.Notes
			}
			rows = append(rows, []string{"", name + "\n" + l.Snapshot.SKU, l.Snapshot.Description, val(l.Quantity), l.Snapshot.UOM, val(l.RequiredDate), state, strings.TrimSpace(details)})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i][1] < rows[j][1] })
	for i := range rows {
		rows[i][0] = fmt.Sprint(i + 1)
	}
	comments := make([]string, len(v.Comments))
	for i, c := range v.Comments {
		comments[i] = strings.ReplaceAll(strings.ReplaceAll(c, "complete_bo_list", "Finalised"), "reopen_bo_list", "Reopened")
	}
	return pdfreport.Render(ctx, pdfreport.Document{Title: "VTPL CORE | Items to purchase", Subtitle: v.Project.Number + " - " + v.Project.Name, Stamp: "Generated " + v.GeneratedAt.In(time.FixedZone("IST", 19800)).Format("02 Jan 2006 15:04 IST"), Columns: []pdfreport.Column{{Title: "No.", Width: 24}, {Title: "Item / SKU", Width: 174}, {Title: "Technical specifications", Width: 248}, {Title: "Quantity", Width: 52}, {Title: "Unit", Width: 38}, {Title: "Site date", Width: 72}, {Title: "Status", Width: 55}, {Title: "Reference / Notes", Width: 130}}, Rows: rows, Notes: comments})
}
