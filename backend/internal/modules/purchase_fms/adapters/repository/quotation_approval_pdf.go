package repository

import (
	"context"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/pdfreport"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"strings"
)

type QuotationApprovalPrinter struct{}

func (QuotationApprovalPrinter) Render(ctx context.Context, s domain.QuotationApprovalsPrint) ([]byte, error) {
	count := len(s.Quotes)
	if s.Attempt != nil {
		count = len(s.Attempt.Proposals)
		if count == 0 {
			count = 1
		}
	}
	if count > 0 && count <= 4 {
		return quotationComparisonPDF(ctx, s)
	}
	rows := [][]string{}
	rowFills := map[int]pdfreport.Fill{}
	notes := []string{}
	title := "Group quotation comparison"
	nominations := map[string]domain.QuotationApprovalsNomination{}
	for _, n := range s.Nominations {
		nominations[n.QuoteKey] = n
	}
	add := func(q domain.QuotationApprovalsAttempt, n domain.QuotationApprovalsNomination) {
		tags := []string{}
		if n.BestPrice {
			tags = append(tags, "Lowest price")
			notes = append(notes, q.Vendor+" - Lowest price: "+n.BestPriceComment)
		}
		if n.Preferred {
			tags = append(tags, "Preferred")
			notes = append(notes, q.Vendor+" - Preferred: "+n.PreferredComment)
		}
		for _, l := range q.Lines {
			budget, difference := "", ""
			if s.Finance {
				if l.Budget != nil {
					budget = *l.Budget
				}
				if l.Difference != nil {
					difference = *l.Difference
				}
			}
			detail := l.Comment
			pricing := ""
			if l.Price != nil {
				pricing = "Unit " + l.Price.Rate + "; discount " + l.Price.DiscountPercent + "%; tax " + l.Price.TaxPercent + "%; charges " + l.Price.Charges
			}
			if l.Months != nil {
				pricing += "\n" + l.Coverage + fmt.Sprintf(" %d months", *l.Months)
			}
			if l.ExecutiveItemNote != "" {
				pricing += "\n" + l.ExecutiveItemNote
			}
			if l.Decision != nil {
				detail = *l.Decision + " - " + detail
			}
			rowFills[len(rows)] = quotationSelectionFill(n)
			rows = append(rows, []string{q.Vendor + "\n" + q.Reference + fmt.Sprintf(" R%d", q.Revision) + "\n" + strings.Join(tags, " / "), l.Item.Description + "\n" + l.Item.SKU + "\n" + pricing, l.Quantity + " " + l.Item.UOM, l.Quoted, budget, difference, detail})
		}
		notes = append(notes, q.Vendor+" - total "+s.Currency+" "+q.Total+" | Valid until "+q.ValidUntil+" | "+q.Terms)
	}
	if s.Attempt != nil {
		title = "Shortlisted quotation proposals"
		a := s.Attempt
		notes = append(notes, "Submission "+a.ID+" | "+a.EffectiveState+" | "+a.Message, "Submitted "+a.SubmittedAt.Format("02 Jan 2006 15:04 UTC")+" by "+a.SubmittedBy, "Executive summary: "+a.Reason, "Finance summary: "+a.ReviewComment)
		if len(a.Proposals) > 0 {
			for _, p := range a.Proposals {
				q := p.Snapshot
				q.QuoteKey = p.QuoteKey
				if a.QuoteKey == p.QuoteKey && a.State != "pending_accounts" {
					q.Lines = a.Lines
				}
				add(q, p.QuotationApprovalsNomination)
			}
		} else {
			add(*a, domain.QuotationApprovalsNomination{})
		}
		if a.ReturnAction != "" {
			notes = append(notes, "Finance return instruction: "+strings.ReplaceAll(a.ReturnAction, "_", " "))
		}
	} else {
		if len(s.Nominations) > 0 {
			title = "Draft shortlisted quotation proposals"
			notes = append(notes, "Draft selection - not submitted for approval")
		}
		for _, q := range s.Quotes {
			a := domain.QuotationApprovalsAttempt{Vendor: q.Vendor, Reference: q.Reference, Revision: q.Revision, Total: q.Total, Terms: q.Terms, ValidUntil: q.ValidUntil}
			for _, l := range q.Lines {
				a.Lines = append(a.Lines, domain.QuotationApprovalsLine{Item: l.Item, Quantity: l.Quantity, Quoted: l.Quoted, Price: &l.Price, Coverage: l.Coverage, Months: l.Months, ExecutiveItemNote: l.Notes})
			}
			n := nominations[q.Key]
			if len(s.Nominations) == 0 && q.Lowest {
				n.BestPrice = true
				n.BestPriceComment = "Lowest eligible landed group total"
			}
			add(a, n)
			if !q.Eligible {
				notes = append(notes, q.Vendor+" R"+fmt.Sprint(q.Revision)+" excluded: "+strings.Join(q.Exclusions, "; "))
			}
		}
	}
	cols := []pdfreport.Column{{Title: "Vendor / revision", Width: 180}, {Title: "Item / specification", Width: 230}, {Title: "Quantity", Width: 55}, {Title: "Amount incl. tax", Width: 85}, {Title: "Budget", Width: 65}, {Title: "Difference", Width: 65}, {Title: "Finance notes", Width: 113}}
	if !s.Finance {
		for i := range rows {
			rows[i] = rows[i][:4]
		}
		cols = cols[:4]
		cols[1].Width = 400
		cols[0].Width = 220
		cols[2].Width = 70
		cols[3].Width = 103
	}
	return pdfreport.Render(ctx, pdfreport.Document{Title: "VTPL CORE | " + title, Subtitle: s.ProjectNumber + " - " + s.ProjectName + " | " + s.GroupName, Stamp: "Generated " + s.GeneratedAt.Format("02 Jan 2006 15:04 UTC"), Columns: cols, Rows: rows, Notes: notes, RowFills: rowFills, Legend: quotationSelectionLegend()})
}
