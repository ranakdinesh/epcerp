package application

import (
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func (s *QuotationApprovalsService) WithPrinter(p ports.QuotationApprovalsPrinter) *QuotationApprovalsService {
	s.printer = p
	return s
}

// Print reads the exact owned revisions. Review budgets are never exposed to Procurement.
func (s *QuotationApprovalsService) Print(ctx context.Context, token, p, b, group, attemptID string, nominations []domain.QuotationApprovalsNomination, keys []string, financeNotes map[string]string) (data []byte, e error) {
	a, e := s.actor(ctx, token, p, b)
	if e != nil {
		return nil, e
	}
	if len(financeNotes) > 0 && attemptID == "" {
		return nil, domain.QuotationApprovalsInvalid("finance_notes", "Draft Finance notes require a frozen submission")
	}
	if s.printer == nil {
		return nil, domain.QuotationApprovalsInvalid("pdf", "Quotation printing is unavailable")
	}
	if _, e = domain.QuotationApprovalsID(group); e != nil {
		return nil, e
	}
	if attemptID != "" {
		if _, e = domain.QuotationApprovalsID(attemptID); e != nil {
			return nil, e
		}
	}
	if len(nominations) > 0 {
		if nominations, e = domain.QuotationApprovalsValidateNominations(nominations); e != nil {
			return nil, e
		}
	}
	var sheet domain.QuotationApprovalsPrint
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, _ ports.QuotationApprovalsFinance) error {
		caps, e := c.Actions(ctx, a, p)
		if e != nil {
			return e
		}
		if len(financeNotes) > 0 && !caps.ReviewRead {
			return domain.QuotationApprovalsForbidden
		}
		if !caps.Read {
			return domain.QuotationApprovalsForbidden
		}
		labels, ok := c.(ports.QuotationApprovalsQueueContext)
		if !ok {
			return domain.QuotationApprovalsMissing
		}
		name, number, e := labels.ProjectLabel(ctx, p)
		if e != nil {
			return e
		}
		source, e := c.Source(ctx, p, b)
		if e != nil {
			return e
		}
		sheet = domain.QuotationApprovalsPrint{ProjectName: name, ProjectNumber: number, GroupID: group, Currency: source.Currency, Finance: caps.ReviewRead, GeneratedAt: s.v.Now(), Nominations: nominations}
		for _, g := range source.Groups {
			if g.ID == group {
				sheet.GroupName = g.Name
			}
		}
		if sheet.GroupName == "" {
			return domain.QuotationApprovalsMissing
		}
		if attemptID != "" {
			ws, e := quotationApprovalsWorkspace(ctx, r, c, a, p, b)
			if e != nil {
				return e
			}
			for _, row := range ws.Attempts {
				if row.ID == attemptID && row.GroupID == group {
					copy := row
					sheet.Attempt = &copy
					break
				}
			}
			if sheet.Attempt == nil {
				return domain.QuotationApprovalsMissing
			}
			if len(financeNotes) > 0 {
				copy, e := domain.QuotationApprovalsDraftFinanceNotes(*sheet.Attempt, financeNotes)
				if e != nil {
					return e
				}
				sheet.Attempt = &copy
			}

		} else {
			if len(nominations) == 0 {
				sheet.Quotes, e = domain.QuotationApprovalsComparisonSheet(source, group, keys)
				return e
			}
			found := map[string]bool{}
			for _, q := range source.Quotes {
				if q.GroupID != group {
					continue
				}
				if len(nominations) > 0 {
					include := false
					for _, n := range nominations {
						if n.QuoteKey == q.Key {
							include = true
							found[n.QuoteKey] = true
						}
					}
					if !include {
						continue
					}
				}
				sheet.Quotes = append(sheet.Quotes, q)
			}
			for _, n := range nominations {
				if !found[n.QuoteKey] {
					return domain.QuotationApprovalsMissing
				}
			}
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return s.printer.Render(ctx, sheet)
}
