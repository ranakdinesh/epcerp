package domain

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

var tscStop = map[string]bool{"the": true, "and": true, "for": true, "this": true, "that": true, "what": true, "any": true, "required": true, "tender": true, "please": true, "from": true, "with": true, "there": true, "which": true, "name": true, "will": true, "where": true, "whether": true, "details": true, "give": true, "are": true, "has": true, "have": true, "its": true, "provide": true, "subject": true, "yes": true, "not": true, "if": true, "is": true, "of": true, "to": true, "in": true, "be": true, "or": true, "as": true, "it": true}

var tscSectionQueries = map[string]string{
	"emd":                "earnest money bid security deposit declaration exemption msme udyam startup surety",
	"tender_fee":         "document fee tender cost processing charges payment exemption msme udyam startup",
	"financial":          "turnover solvency net worth working capital audited bank certificate",
	"electrical_licence": "electrical licence license contractor voltage kv endorsement",
	"similar_work":       "similar work experience completion eligibility qualification authorization oem",
	"security":           "performance security guarantee retention defect liability warranty surety",
	"payment":            "payment milestone invoice employer release running account bill",
	"risk":               "penalty liquidated damages escalation labour variation",
	"other":              "forms annexure technical qualification price conditions",
}
var tscQuestionQueries = map[string]string{
	"particulars.q01": "department employer authority organisation organization purchaser owner client corporation board limited",
	"particulars.q02": "name work title project description scope invitation notice nit",
	"particulars.q03": "location site district state city place project",
	"particulars.q04": "estimated cost value amount including inclusive gst taxes",
	"particulars.q05": "estimated cost value amount excluding exclusive gst taxes",
	"particulars.q06": "reverse auction ra electronic bidding",
	"particulars.q07": "completion duration period months days schedule execution",
	"particulars.q08": "prebid pre-bid meeting conference date time",
	"particulars.q09": "submission closing deadline due date time bid opening",
	"particulars.q10": "portal website https eprocure eprocurement online",
	"particulars.q11": "site visit inspection mandatory compulsory",
	"particulars.q12": "site visit contact telephone mobile email officer",
	"particulars.q13": "physical original hard copy offline envelope dispatch delivery",
	"particulars.q14": "bid offer validity valid days",
	"particulars.q15": "joint venture consortium mou jv association",
	"particulars.q16": "works goods services contract supply epc",
	"financial.q02":   "average annual turnover financial years audited",
	"financial.q07":   "solvency bank certificate amount",
	"financial.q08":   "net worth positive negative certificate",
	"financial.q09":   "working capital liquid assets credit facilities",
	"security.q03":    "defect liability maintenance dlp period",
	"security.q04":    "led luminaire light warranty guarantee period",
}

func tscWords(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	out := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) < 2 || tscStop[word] {
			continue
		}
		if len(word) > 4 && strings.HasSuffix(word, "s") {
			word = strings.TrimSuffix(word, "s")
		}
		switch word {
		case "licence":
			word = "license"
		case "authorisation":
			word = "authorization"
		case "labour":
			word = "labor"
		}
		out = append(out, word)
	}
	return out
}

// RetrieveTSCEvidence scores the complete authorised snapshot with BM25, exact
// phrases and question-specific aliases. Duplicate text cannot crowd out other
// clauses. Context expansion retains neighbouring conditions and table notes.
// It never treats a retrieval miss as proof of absence from the tender package.
func RetrieveTSCEvidence(q TSCQuestion, all []TSCPassage) []TSCPassage {
	if len(all) == 0 {
		return []TSCPassage{}
	}
	query := map[string]float64{}
	for _, word := range tscWords(tscSectionQueries[q.Section]) {
		query[word] = 0.25
	}
	for _, word := range tscWords(tscQuestionQueries[q.ID]) {
		query[word] = 1.2
	}
	for _, word := range tscWords(q.Text) {
		query[word] = 2
	}
	type row struct {
		index  int
		tf     map[string]int
		length int
		score  float64
	}
	rows := make([]row, 0, len(all))
	df := map[string]int{}
	total := 0
	for i, p := range all {
		if IsTSCReferenceSource(p.Filename) {
			continue
		}
		text := p.SearchText
		if text == "" {
			text = p.Text
		}
		words := tscWords(p.Heading + " " + text)
		tf := map[string]int{}
		for _, word := range words {
			tf[word]++
		}
		for word := range tf {
			if _, ok := query[word]; ok {
				df[word]++
			}
		}
		rows = append(rows, row{index: i, tf: tf, length: len(words)})
		total += len(words)
	}
	if len(rows) == 0 {
		return []TSCPassage{}
	}
	average := float64(total) / float64(len(rows))
	if average < 1 {
		average = 1
	}
	for i := range rows {
		r := &rows[i]
		for word, weight := range query {
			tf := float64(r.tf[word])
			if tf == 0 {
				continue
			}
			idf := math.Log(1 + (float64(len(rows)-df[word])+0.5)/(float64(df[word])+0.5))
			r.score += weight * idf * tf * 2.2 / (tf + 1.2*(0.25+0.75*float64(r.length)/average))
		}
		p := all[r.index]
		// Classification is a hint, never permission or automatic source precedence.
		if p.DocumentType == "notice" && q.Section == "particulars" {
			r.score *= 1.15
		}
		if p.DocumentType == "corrigendum" {
			r.score *= 1.1
		}
		if p.Suppressed {
			r.score *= 0.15
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].score > rows[j].score })
	out := []TSCPassage{}
	seen := map[string]bool{}
	indices := map[int]bool{}
	counts := map[string]int{}
	budget := 0
	add := func(index int) bool {
		p := all[index]
		if IsTSCReferenceSource(p.Filename) {
			return false
		}
		key := strings.Join(strings.Fields(p.Text), " ")
		if key == "" || seen[key] || indices[index] || len(out) >= 16 || budget+len(p.Text) > 14000 {
			return false
		}
		out = append(out, p)
		seen[key] = true
		indices[index] = true
		counts[p.DocumentID]++
		budget += len(p.Text)
		return true
	}
	// First preserve source diversity; then fill remaining relevance-ranked slots.
	for _, r := range rows {
		if len(out) >= 8 {
			break
		}
		if r.score > 0 && counts[all[r.index].DocumentID] < 3 {
			add(r.index)
		}
	}
	for _, r := range rows {
		if r.score > 0 {
			add(r.index)
		}
		if len(out) >= 12 {
			break
		}
	}
	// Add adjacent fragments in original document order, including split clauses
	// and table continuations. Expansion is still inside the same source revision.
	anchors := append([]TSCPassage{}, out...)
	for _, anchor := range anchors {
		for i, p := range all {
			if p.ID != anchor.ID {
				continue
			}
			for _, j := range []int{i - 1, i + 1} {
				if j >= 0 && j < len(all) && all[j].DocumentID == p.DocumentID && all[j].Revision == p.Revision && all[j].Segment == p.Segment && all[j].Alternative == p.Alternative {
					add(j)
				}
			}
			break
		}
	}
	return out
}

func TSCSearchDiagnostic(q TSCQuestion, passages []TSCPassage) TSCQuestionDiagnostic {
	selected := RetrieveTSCEvidence(q, passages)
	docs := map[string]bool{}
	for _, p := range selected {
		docs[p.DocumentID] = true
	}
	return TSCQuestionDiagnostic{QuestionID: q.ID, Candidates: len(selected), Documents: len(docs)}
}
