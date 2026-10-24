package repository

import (
	"context"
	"fmt"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/pdfreport"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"strings"
)

// Group sheets keep each offer in its own comparison column. Snapshot exports
// retain executive reasons and the exact submitted budget/price/Finance notes.
func quotationComparisonPDF(ctx context.Context, s domain.QuotationApprovalsPrint) ([]byte, error) {
	sheet, e := quotationComparisonSheet(s)
	if e != nil {
		return nil, e
	}
	return pdfreport.Render(ctx, sheet)
}
func quotationComparisonSheet(s domain.QuotationApprovalsPrint) (pdfreport.Document, error) {
	quotes := []domain.QuotationApprovalsAttempt{}
	nominations := map[string]domain.QuotationApprovalsNomination{}
	notes := []string{}
	title := "Group quotation comparison"
	for _, n := range s.Nominations {
		nominations[n.QuoteKey] = n
	}
	if len(s.Nominations) > 0 {
		title = "Draft shortlisted proposals"
		notes = append(notes, "Draft selection - not submitted for approval")
	}
	if s.Attempt != nil {
		a := s.Attempt
		title = "Shortlisted proposals and Finance review"
		notes = append(notes, "Submission "+a.ID+" | "+a.EffectiveState+" | "+a.Message, "Submitted "+a.SubmittedAt.Format("02 Jan 2006 15:04 UTC")+" by "+a.SubmittedBy, "Executive summary: "+a.Reason)
		if a.ReviewComment != "" {
			notes = append(notes, "Finance decision: "+a.ReviewComment)
		}
		if a.ReturnAction != "" {
			notes = append(notes, "Return instruction: "+strings.ReplaceAll(a.ReturnAction, "_", " "))
		}
		if len(a.Proposals) > 0 {
			for _, p := range a.Proposals {
				q := p.Snapshot
				q.QuoteKey = p.QuoteKey
				if a.QuoteKey == p.QuoteKey && a.State != "pending_accounts" {
					q.Lines = a.Lines
				}
				quotes = append(quotes, q)
				nominations[p.QuoteKey] = p.QuotationApprovalsNomination
			}
		} else {
			quotes = append(quotes, *a)
		}
	} else {
		for _, q := range s.Quotes {
			a := domain.QuotationApprovalsAttempt{QuoteKey: q.Key, Vendor: q.Vendor, Reference: q.Reference, Revision: q.Revision, Total: q.Total, Terms: q.Terms, ValidUntil: q.ValidUntil}
			for _, l := range q.Lines {
				a.Lines = append(a.Lines, domain.QuotationApprovalsLine{LineID: l.LineID, Item: l.Item, Quantity: l.Quantity, Quoted: l.Quoted, Price: &l.Price, Coverage: l.Coverage, Months: l.Months, ExecutiveItemNote: l.Notes})
			}
			quotes = append(quotes, a)
			if !q.Eligible {
				notes = append(notes, q.Vendor+" - excluded: "+strings.Join(q.Exclusions, "; "))
			}
		}
	}
	if len(quotes) == 0 {
		return pdfreport.Document{}, domain.QuotationApprovalsMissing
	}
	columnFills := map[int]pdfreport.Fill{}
	cols := []pdfreport.Column{{Title: "Item / specification", Width: 195}, {Title: "Quantity", Width: 55}}
	budget := s.Finance && s.Attempt != nil
	if budget {
		cols = append(cols, pdfreport.Column{Title: "Budget", Width: 72})
	}
	remaining := 793. - 250.
	if budget {
		remaining -= 72
	}
	for i, q := range quotes {
		cols = append(cols, pdfreport.Column{Title: q.Vendor + fmt.Sprintf("\nRevision %d", q.Revision), Width: remaining / float64(len(quotes))})
		n := nominations[q.QuoteKey]
		if len(s.Nominations) == 0 && s.Attempt == nil {
			for _, source := range s.Quotes {
				if source.Key == q.QuoteKey && source.Lowest {
					n.BestPrice = true
					n.BestPriceComment = "Lowest eligible landed group total"
				}
			}
		}
		columnFills[len(cols)-1] = quotationSelectionFill(n)
		labels := []string{}
		if n.BestPrice {
			labels = append(labels, "Lowest price")
			notes = append(notes, q.Vendor+" - Lowest price reason: "+n.BestPriceComment)
		}
		if n.Preferred {
			labels = append(labels, "Preferred")
			notes = append(notes, q.Vendor+" - Preferred reason: "+n.PreferredComment)
		}
		if len(labels) > 0 {
			cols[len(cols)-1].Title += "\n" + strings.Join(labels, " + ")
		}
		notes = append(notes, fmt.Sprintf("Offer %d: %s | %s R%d | %s", i+1, q.Vendor, q.Reference, q.Revision, strings.Join(labels, " / ")), q.Terms+" | Valid until "+q.ValidUntil)
	}
	rows := [][]string{}
	seen := map[string]bool{}
	for _, q := range quotes {
		for _, l := range q.Lines {
			if seen[l.LineID] {
				continue
			}
			seen[l.LineID] = true
			row := []string{l.Item.Description + "\n" + l.Item.SKU, l.Quantity + " " + l.Item.UOM}
			if budget {
				b := "Not allocated"
				if l.Budget != nil {
					b = *l.Budget
				}
				row = append(row, b)
			}
			for _, offer := range quotes {
				cell := "Not quoted"
				for _, priced := range offer.Lines {
					if priced.LineID != l.LineID {
						continue
					}
					cell = s.Currency + " " + priced.Quoted
					if priced.Price != nil {
						cell += "\nUnit: " + priced.Price.Rate + "\nDiscount: " + priced.Price.DiscountPercent + "%; Tax: " + priced.Price.TaxPercent + "%\nCharges: " + priced.Price.Charges
					}
					if priced.Months != nil {
						cell += "\n" + priced.Coverage + fmt.Sprintf(" %d months", *priced.Months)
					}
					if budget && priced.Difference != nil {
						cell += "\nDifference: " + *priced.Difference
					}
					if priced.ExecutiveItemNote != "" {
						cell += "\nItem note: " + priced.ExecutiveItemNote
					}
					if s.Finance && priced.Comment != "" {
						cell += "\nFinance: " + priced.Comment
					}
				}
				row = append(row, cell)
			}
			rows = append(rows, row)
		}
	}
	total := []string{"Group total including tax / charges", ""}
	if budget {
		total = append(total, "")
	}
	for _, q := range quotes {
		total = append(total, s.Currency+" "+q.Total)
	}
	rows = append(rows, total)
	return pdfreport.Document{Title: "VTPL CORE | " + title, Subtitle: s.ProjectNumber + " - " + s.ProjectName + " | " + s.GroupName, Stamp: "Generated " + s.GeneratedAt.Format("02 Jan 2006 15:04 UTC"), Columns: cols, Rows: rows, Notes: notes, ColumnFills: columnFills, Legend: quotationSelectionLegend()}, nil
}
