package domain

import "strings"

func TSCResearchQuestion(q TSCQuestion, query string) TSCQuestion {
	q.Text += "\nReviewer search terms: " + strings.TrimSpace(query)
	return q
}

// Names are an explicit conservative exclusion, never a claim that other files
// have been fully classified. Sections and amendment scope remain reviewed work.
func IsTSCReferenceSource(path string) bool {
	name := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(path, "_", " "), "-", " "))
	return strings.Contains(name, "tender selection") || strings.Contains(name, "tsc format") || strings.Contains(name, "completed tsc")
}
