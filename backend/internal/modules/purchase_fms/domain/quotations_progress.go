package domain

// Progress rows contain saved source/pricing coverage, not price amounts or authority.
type QuotationsProgressLine struct {
	Source QuotationsSourceLine
	Priced bool
}
type QuotationsProgressQuote struct {
	DocumentID, ReceivedDate string
	Revision                 int32
	Lines                    []QuotationsProgressLine
}
type QuotationsProgress struct {
	ProjectID, BOID string
	Quotes          []QuotationsProgressQuote
}

func QuotationsProgressFacts(r *QuotationsRecord, p, b string) (QuotationsProgress, error) {
	out := QuotationsProgress{ProjectID: p, BOID: b, Quotes: []QuotationsProgressQuote{}}
	if r == nil || !r.Active {
		return out, nil
	}
	if r.ProjectID != p || r.BOID != b {
		return out, QuotationsInvalid("source", "Quotation progress must match project and BO")
	}
	seen := map[string]bool{}
	for _, q := range r.Quotes {
		if seen[q.DocumentID] || q.DocumentID == "" || q.Revision < 1 {
			return QuotationsProgress{}, QuotationsInvalid("quotes", "Use one saved current quotation revision per document")
		}
		seen[q.DocumentID] = true
		row := QuotationsProgressQuote{DocumentID: q.DocumentID, ReceivedDate: q.ReceivedDate, Revision: q.Revision, Lines: []QuotationsProgressLine{}}
		for _, line := range q.Calculation.Lines {
			source := line.Source
			source.Quantity = quotationsCopyString(source.Quantity)
			source.RequiredDate = quotationsCopyString(source.RequiredDate)
			source.ItemID = quotationsCopyString(source.ItemID)
			source.DemandID = quotationsCopyString(source.DemandID)
			row.Lines = append(row.Lines, QuotationsProgressLine{Source: source, Priced: line.Landed != nil && line.LineID == source.ID})
		}
		out.Quotes = append(out.Quotes, row)
	}
	return out, nil
}
func quotationsCopyString(v *string) *string {
	if v == nil {
		return nil
	}
	copy := *v
	return &copy
}
