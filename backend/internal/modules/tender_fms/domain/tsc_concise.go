package domain

import (
	"regexp"
	"strings"
)

var tscSentenceBoundary = regexp.MustCompile(`[.!?]\s+`)
var tscAmount = regexp.MustCompile(`(?i)(?:INR|Rs\.?|₹)\s*[0-9][0-9,]*(?:\.[0-9]+)?(?:\s*(?:crores?|lakhs?|lacs?|million|billion))?`)
var tscEvidenceComment = regexp.MustCompile(`(?i)^(?:this is (?:stated|mentioned|specified)|this (?:is|can be) (?:found|confirmed)|(?:the )?(?:document|source|passage|clause) (?:states|mentions|specifies|confirms)|for (?:example|instance)|as (?:stated|mentioned)|see (?:source|page|clause))\b`)
var tscInlineSource = regexp.MustCompile(`\s*\(S[0-9]+(?:\s*[,;]\s*S[0-9]+)*\)`)

// A response state is metadata, never an answer to a checklist question.
func TSCPlaceholder(text string) bool {
	text = strings.ToLower(strings.Trim(strings.TrimSpace(text), ".:;* _-\"`"))
	text = strings.Join(strings.Fields(strings.ReplaceAll(text, "_", " ")), " ")
	switch text {
	case "", "answered", "answer", "not found", "needs review", "conflicting", "conflicting information", "unknown", "not available", "not yet extracted":
		return true
	}
	return false
}

// CompactTSCText removes presentation commentary, never truncates a condition by
// character count, and leaves original citations untouched. Value and RA fields
// request their direct value/applicability rather than a narrated explanation.
func CompactTSCText(question, text string) string {
	text = strings.Join(strings.Fields(text), " ")
	text = tscInlineSource.ReplaceAllString(text, "")
	text = strings.TrimSpace(strings.TrimPrefix(text, "Answered:"))
	parts := tscSentenceBoundary.Split(text, -1)
	if len(parts) == 0 {
		return text
	}
	first := strings.TrimSpace(parts[0])
	if question == "particulars.q05" {
		amounts := tscAmount.FindAllString(first, -1)
		if len(amounts) == 1 && !strings.ContainsAny(first, "<>≤≥") && !regexp.MustCompile(`(?i)\b(?:between|from|approximately|about|upto|up to|at least|more than|less than|or|excluding|including)\b`).MatchString(first) {
			return strings.TrimSpace(amounts[0])
		}
	}
	if question == "particulars.q06" && strings.HasPrefix(strings.ToLower(first), "reverse auction (ra) is applicable to ") {
		first = "Yes; " + first[len("Reverse Auction (RA) is applicable to "):]
		first = strings.ReplaceAll(first, "arranged in order of prices from lowest to highest", "ranked by price (lowest first)")
		// The full auction procedure remains in supporting evidence.
		return strings.TrimRight(first, ".")
	}
	kept := []string{}
	for i, part := range parts {
		if part = strings.TrimSpace(part); part != "" && (i == 0 || !tscEvidenceComment.MatchString(part)) {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, "; ")
}

func ConciseTSCAnswer(a TSCAnswer) TSCAnswer {
	if a.Status == "answered" && !a.Accepted && a.Origin != "manual" {
		a.Text = CompactTSCText(a.QuestionID, a.Text)
	}
	return a
}
