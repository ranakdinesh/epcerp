package domain

import (
	"regexp"
	"strings"
)

// Only explicitly labelled tender particulars qualify for deterministic capture.
// Financial eligibility, exemptions and business judgements still use reviewed
// source interpretation; the label alone can never establish those decisions.
var tscFactLabels = map[string]*regexp.Regexp{
	"particulars.q01": regexp.MustCompile(`(?i)^\s*(?:name of (?:the )?department|department|employer|name of (?:the )?employer|tender inviting authority)\s*[:：]\s*(\S.*)$`),
	"particulars.q02": regexp.MustCompile(`(?i)^\s*(?:name of (?:the )?work|name of (?:the )?project|description of work|work name)\s*[:：]\s*(\S.*)$`),
	"particulars.q03": regexp.MustCompile(`(?i)^\s*(?:location of (?:the )?work|project location|work location|location of (?:the )?project)\s*[:：]\s*(\S.*)$`),
	"particulars.q07": regexp.MustCompile(`(?i)^\s*(?:completion period|period of completion|time for completion|completion duration|time allowed for completion)\s*[:：]\s*(\S.*)$`),
	"particulars.q08": regexp.MustCompile(`(?i)^\s*(?:pre[- ]?bid meeting date|date of pre[- ]?bid meeting|pre[- ]?bid date)\s*[:：]\s*(\S.*)$`),
	"particulars.q09": regexp.MustCompile(`(?i)^\s*(?:last date (?:and time )?for (?:bid )?submission|bid submission (?:end date|deadline|closing date)|last date of (?:bid )?submission)\s*[:：]\s*(\S.*)$`),
	"particulars.q10": regexp.MustCompile(`(?i)^\s*(?:e[- ]?tendering portal|e[- ]?procurement portal|tender portal)\s*[:：]\s*(\S.*)$`),
	"particulars.q14": regexp.MustCompile(`(?i)^\s*(?:bid validity|offer validity|validity of (?:the )?(?:bid|offer)|period of (?:bid|offer) validity)\s*[:：]\s*(\S.*)$`),
}

// CollectTSCCandidates is a document-wide first pass over all blocks, not a
// top-k search. It retains disagreements rather than inventing precedence.
func CollectTSCCandidates(in TSCInput) map[string]TSCAnswer {
	result := map[string]TSCAnswer{}
	for _, q := range in.Questions {
		label, ok := tscFactLabels[q.ID]
		if !ok {
			continue
		}
		values := map[string]bool{}
		answer := TSCAnswer{QuestionID: q.ID, Status: "answered", Citations: []TSCCitation{}, Origin: "labelled_source_candidate"}
		valueList := []string{}
		for _, p := range in.Passages {
			if IsTSCReferenceSource(p.Filename) {
				continue
			}
			if p.Suppressed || p.Alternative || (p.Confidence != nil && *p.Confidence < 70) {
				continue
			}
			for _, line := range strings.Split(p.Text, "\n") {
				match := label.FindStringSubmatch(line)
				if len(match) != 2 {
					continue
				}
				value := strings.TrimSpace(match[1])
				key := strings.ToLower(strings.Join(strings.Fields(value), " "))
				if len(value) < 3 || len(value) > 1000 || strings.ContainsAny(value, "_{}") || values[key] {
					continue
				}
				values[key] = true
				valueList = append(valueList, value)
				if len(answer.Citations) < 8 {
					answer.Citations = append(answer.Citations, TSCCitation{SourceID: p.ID, Quote: p.Text})
				}
			}
		}
		if len(valueList) == 0 {
			continue
		}
		answer.Text = strings.Join(valueList, "\n")
		if len(valueList) > 1 {
			answer.Status = "conflicting"
			answer.Text = "Different explicitly labelled source values require applicability/precedence review:\n" + answer.Text
		}
		if len(valueList) > 8 || len(answer.Text) > 7500 {
			answer.Status = "needs_review"
			answer.Text = "More than eight distinct labelled values were found. Review all source occurrences before selecting the applicable value."
		}
		if ValidateTSCAnswer(answer, in) == nil {
			result[q.ID] = answer
		}
	}
	return result
}
