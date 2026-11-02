package domain

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

type tscFactField struct {
	id     string
	label  *regexp.Regexp
	review bool
}

// Explicit labels establish source facts, never company eligibility. Clauses and
// compound questions remain reviewable excerpts, not applicability decisions.
var tscFactFields = []tscFactField{
	factField("particulars.q01", `(?:name of (?:the )?)?(?:department|employer|tender inviting authority|organisation|organization)|department name`, false),
	factField("particulars.q02", `name of (?:the )?(?:work|project)|description of (?:the )?work|work name|project name`, false),
	factField("particulars.q03", `(?:location|site|place) of (?:the )?(?:work|project)|(?:work|project|site) location|location where the work will be executed`, false),
	factField("particulars.q04", `(?:estimated (?:cost|value)|tender value|cost of (?:the )?work)(?:\s*\(?\s*(?:including|inclusive of|with) GST\s*\)?)?`, false),
	factField("particulars.q05", `(?:estimated (?:cost|value)|tender value|cost of (?:the )?work)(?:\s*\(?\s*(?:excluding|exclusive of|without) GST\s*\)?)?`, false),
	factField("particulars.q07", `completion period|period of completion|time for completion|completion duration|time allowed for completion|period of contract`, false),
	factField("particulars.q08", `(?:date (?:and time )?of |date (?:and time )?for )?pre[- ]?bid (?:meeting|conference)(?: date(?: and time)?| due date(?: and time)?)?|pre[- ]?bid date|pre[- ]?bid due date(?: (?:and|&) time)?`, false),
	factField("particulars.q09", `(?:last date(?: and time)? (?:for|of)|date (?:and time )?of closing of) (?:bid submission|submission of (?:e[- ]?tenders?(?: for submission of bid)?|bids?)|e[- ]?tenders? for submission of bids?)|bid submission (?:end date|deadline|closing date)|tender due date(?: (?:and|&) time)?`, false),
	factField("particulars.q10", `e[- ]?tendering portal|e[- ]?procurement portal|tender portal|mode of tender`, false),
	factField("particulars.q14", `(?:bid|offer) validity|validity of (?:the )?(?:bid|offer|tender)|period of (?:bid|offer|tender) validity`, false),
	factField("particulars.q16", `type of contract|contract type|procurement category|tender category`, false),
	factField("tender_fee.q05", `tender fees?|tender document (?:fee|cost)|cost of tender documents?|document fee`, true),
	factField("emd.q05", `earnest money(?: deposit)?(?:\s*\(EMD\))?|EMD(?: amount)?|bid security(?: amount)?`, true),
	factField("similar_work.q03", `definition of similar work|similar work definition`, true),
	factField("financial.q02", `(?:minimum )?average annual (?:financial )?turnover(?: requirement)?`, true),
	factField("financial.q07", `(?:bank )?solvency(?: certificate)?(?: requirement)?`, true),
	factField("financial.q08", `(?:minimum )?net worth(?: requirement)?`, true),
	factField("financial.q09", `(?:minimum )?working capital(?: requirement)?`, true),
	factField("security.q02", `performance security(?: deposit| guarantee)?|performance bank guarantee|security deposit(?: percentage)?`, true),
	factField("security.q03", `defect(?:s)? liability period|DLP`, true),
	factField("payment.q01", `(?:terms of payment|payment terms)(?: and milestones)?|payment milestones`, true),
}

func factField(id, label string, review bool) tscFactField {
	return tscFactField{id, regexp.MustCompile(`(?i)^(?:` + label + `)$`), review}
}

var tscFactSerial = regexp.MustCompile(`^\s*(?:\d+(?:\.\d+)*[.)]?|[a-zA-Z][.)])\s+`)
var tscFactSpaces = regexp.MustCompile(`[ \t]{2,}`)
var tscFactDuration = regexp.MustCompile(`(?i)\b(?:[0-9]+(?:\.[0-9]+)?|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|eighteen|twenty|thirty|ninety|hundred)\s*(?:\([^)\n]{1,80}\)\s*)?(?:working\s+|calendar\s+)?(?:days?|weeks?|months?|years?)\b`)
var tscFactDate = regexp.MustCompile(`(?i)\b(?:\d{1,4}[./-]\d{1,2}[./-]\d{1,4}|\d{1,2}\s*(?:jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s*,?\s*\d{4}|(?:jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+\d{1,2},?\s+\d{4})\b`)
var tscFactWithGST = regexp.MustCompile(`(?i)\b(?:including|inclusive(?: of)?|with)\s+(?:the\s+)?GST\b`)
var tscFactWithoutGST = regexp.MustCompile(`(?i)\b(?:excluding|exclusive(?: of)?|without)\s+(?:the\s+)?GST\b`)
var tscFactContract = regexp.MustCompile(`(?i)\b(?:works|goods|services)\b`)

type tscFactLine struct {
	text                string
	passage, start, end int
}
type tscFactRow struct {
	label, value string
	first, last  int
	review       bool
}

func tscFactLabel(s string) string {
	s = strings.Trim(strings.TrimSpace(s), "|*# ")
	s = strings.Trim(tscFactSerial.ReplaceAllString(s, ""), "|*# ")
	s = strings.ReplaceAll(s, "&", " and ")
	return strings.TrimRight(strings.Join(strings.Fields(s), " "), ":：.*? ")
}
func tscKnownFactLabel(s string) bool {
	s = tscFactLabel(s)
	for _, f := range tscFactFields {
		if f.label.MatchString(s) {
			return true
		}
	}
	return false
}
func tscEligibleFact(p TSCPassage) bool {
	return !IsTSCReferenceSource(p.Filename) && !p.Suppressed && !p.Alternative && (p.Confidence == nil || *p.Confidence >= 70)
}
func tscFactLines(passages []TSCPassage, indexes []int) []tscFactLine {
	var lines []tscFactLine
	for _, index := range indexes {
		offset := 0
		parts := strings.Split(passages[index].Text, "\n")
		for n, line := range parts {
			if n == len(parts)-1 && line == "" {
				break
			}
			lines = append(lines, tscFactLine{line, index, offset, offset + len(line)})
			offset += len(line) + 1
		}
	}
	return lines
}

// Infer a value column only from recognised labels with an actual column gap.
// Unknown rows still terminate the preceding value, preventing field bleed.
func tscAlignedFactRows(lines []tscFactLine) ([]tscFactRow, []bool) {
	counts := map[int]int{}
	for _, line := range lines {
		if strings.ContainsAny(line.text, "|\t") {
			continue
		}
		for _, gap := range tscFactSpaces.FindAllStringIndex(line.text, -1) {
			if gap[1] >= 16 && gap[1] <= 100 && tscKnownFactLabel(line.text[:gap[0]]) && strings.TrimSpace(line.text[gap[1]:]) != "" {
				counts[gap[1]]++
			}
		}
	}
	column, best := 0, 0
	for c, n := range counts {
		if n > best || n == best && (column == 0 || c < column) {
			column, best = c, n
		}
	}
	covered := make([]bool, len(lines))
	var rows []tscFactRow
	if column == 0 {
		return rows, covered
	}
	current := tscFactRow{first: -1}
	flush := func() {
		if current.first >= 0 && tscKnownFactLabel(current.label) && strings.TrimSpace(current.value) != "" {
			rows = append(rows, current)
			for i := current.first; i <= current.last; i++ {
				covered[i] = true
			}
		}
		current = tscFactRow{first: -1}
	}
	for i, line := range lines {
		raw := line.text
		if strings.ContainsAny(raw, "|\t") || len(raw) > column && raw[column-1] != ' ' && raw[column] != ' ' {
			flush()
			continue
		}
		left, right := raw, ""
		if len(raw) > column {
			left, right = raw[:column], raw[column:]
		}
		left, right = strings.TrimSpace(left), strings.TrimSpace(right)
		if strings.Contains(left, "://") {
			flush()
			continue
		}
		if left == "" && right == "" {
			flush()
			continue
		}
		if left != "" {
			if current.first >= 0 && (tscKnownFactLabel(left) || current.value != "" &&
				(tscKnownFactLabel(current.label) || !tscKnownFactLabel(current.label+" "+left))) {
				flush()
			}
			if current.first < 0 {
				current.first = i
			}
			current.label = strings.TrimSpace(current.label + " " + left)
		}
		if current.first >= 0 {
			current.last = i
			if right != "" {
				current.value = strings.TrimSpace(current.value + " " + right)
			}
		}
	}
	flush()
	return rows, covered
}
func tscSplitFactLine(s string) (label, value string, ok bool) {
	for _, separator := range []string{"|", "\t"} {
		if !strings.Contains(s, separator) {
			continue
		}
		cells := strings.Split(strings.Trim(s, "| \r"), separator)
		for i := 0; i < len(cells)-1; i++ {
			candidate := strings.Join(cells[:i+1], " ")
			if tscKnownFactLabel(candidate) {
				return candidate, strings.TrimSpace(strings.Join(cells[i+1:], " ")), true
			}
		}
	}
	for i, r := range s {
		if r == ':' || r == '：' {
			if tscKnownFactLabel(s[:i]) {
				return s[:i], strings.TrimSpace(s[i+utf8.RuneLen(r):]), true
			}
		}
	}
	for _, gap := range tscFactSpaces.FindAllStringIndex(s, -1) {
		if tscKnownFactLabel(s[:gap[0]]) {
			return s[:gap[0]], strings.TrimSpace(s[gap[1]:]), true
		}
	}
	if tscKnownFactLabel(s) {
		return s, "", true
	}
	return "", "", false
}
func tscFactRows(lines []tscFactLine) []tscFactRow {
	rows, covered := tscAlignedFactRows(lines)
	for i := 0; i < len(lines); i++ {
		if covered[i] {
			continue
		}
		label, value, ok := tscSplitFactLine(lines[i].text)
		last := i
		if !ok && i+1 < len(lines) && !covered[i+1] && lines[i].passage == lines[i+1].passage {
			label, value, ok = tscSplitFactLine(strings.TrimSpace(lines[i].text) + " " + strings.TrimSpace(lines[i+1].text))
			if ok {
				last = i + 1
			}
		}
		if !ok {
			continue
		}
		for j := last + 1; j < len(lines) && lines[j].passage == lines[i].passage && !covered[j]; j++ {
			next := lines[j].text
			if strings.TrimSpace(next) == "" {
				break
			}
			if _, _, known := tscSplitFactLine(next); known {
				break
			}
			indent := len(next) - len(strings.TrimLeft(next, " \t"))
			if value != "" && indent < 2 {
				break
			}
			if strings.Contains(next, "|") || len(tscFactSpaces.FindAllStringIndex(strings.TrimSpace(next), -1)) > 0 {
				break
			}
			value = strings.TrimSpace(value + " " + strings.TrimSpace(next))
			last = j
		}
		if value != "" {
			rows = append(rows, tscFactRow{label: label, value: value, first: i, last: last})
		}
		i = last
	}
	return rows
}
func tscFactCitations(lines []tscFactLine, row tscFactRow, passages []TSCPassage) []TSCCitation {
	var out []TSCCitation
	for i := row.first; i <= row.last; {
		first, last := lines[i], lines[i]
		j := i + 1
		for j <= row.last && lines[j].passage == first.passage {
			last = lines[j]
			j++
		}
		quote := passages[first.passage].Text[first.start:last.end]
		if len(quote) < 5 || len(quote) > 2000 {
			return nil
		}
		out = append(out, TSCCitation{SourceID: passages[first.passage].ID, Quote: quote})
		i = j
	}
	return out
}
func tscFactValueAllowed(f tscFactField, label, value string) bool {
	if len(value) < 3 || len(value) > 1000 || strings.ContainsAny(value, "_{}") || TSCPlaceholder(value) {
		return false
	}
	switch f.id {
	case "particulars.q04", "particulars.q05":
		text := label + " " + value
		with, without := tscFactWithGST.MatchString(text), tscFactWithoutGST.MatchString(text)
		return tscAmount.MatchString(value) && with != without && (f.id == "particulars.q04") == with
	case "particulars.q07", "particulars.q14", "security.q03":
		return tscFactDuration.MatchString(value)
	case "particulars.q08", "particulars.q09":
		return tscFactDate.MatchString(value)
	case "particulars.q10":
		return !strings.EqualFold(tscFactLabel(label), "mode of tender") || strings.Contains(strings.ToLower(value), "http")
	case "particulars.q16":
		return tscFactContract.MatchString(value)
	case "tender_fee.q05", "emd.q05":
		return tscAmount.MatchString(value) || strings.EqualFold(value, "nil")
	}
	return true
}

// Walk the full authorised snapshot. Exact row excerpts stay tied to original
// passages; neither ranking nor a model supplies values. Conflicts remain visible.
func CollectTSCCandidates(in TSCInput) map[string]TSCAnswer {
	type candidate struct {
		value     string
		citations []TSCCitation
		review    bool
	}
	found := map[string][]candidate{}
	wanted := map[string]bool{}
	for _, q := range in.Questions {
		wanted[q.ID] = true
	}
	collect := func(lines []tscFactLine, rows []tscFactRow) {
		for _, row := range rows {
			label, value := tscFactLabel(row.label), strings.Join(strings.Fields(row.value), " ")
			for _, f := range tscFactFields {
				if !wanted[f.id] || !f.label.MatchString(label) || !tscFactValueAllowed(f, label, value) {
					continue
				}
				citations := tscFactCitations(lines, row, in.Passages)
				if len(citations) == 0 {
					continue
				}
				found[f.id] = append(found[f.id], candidate{value, citations, f.review || row.review})
			}
		}
	}
	for i := 0; i < len(in.Passages); i++ {
		p := in.Passages[i]
		if !tscEligibleFact(p) {
			continue
		}
		indexes := []int{i}
		if p.Kind == "table" {
			for j := i + 1; j < len(in.Passages); j++ {
				prev, next := in.Passages[j-1], in.Passages[j]
				if !tscEligibleFact(next) || next.Kind != "table" || !tscAdjacentFact(prev, next) {
					break
				}
				indexes = append(indexes, j)
			}
		}
		lines := tscFactLines(in.Passages, indexes)
		collect(lines, tscFactRows(lines))
		i = indexes[len(indexes)-1]
		if p.Kind == "heading" && tscKnownFactLabel(p.Text) && i+1 < len(in.Passages) {
			next := in.Passages[i+1]
			if tscEligibleFact(next) && next.Kind == "paragraph" && tscAdjacentFact(p, next) && len(next.Text) <= 1000 {
				lines := tscFactLines(in.Passages, []int{i, i + 1})
				collect(lines, []tscFactRow{{label: p.Text, value: next.Text, first: 0, last: len(lines) - 1, review: true}})
			}
		}
	}
	result := map[string]TSCAnswer{}
	for id, rows := range found {
		a := TSCAnswer{QuestionID: id, Status: "answered", Origin: "labelled_source_candidate", Citations: []TSCCitation{}}
		values := map[string]bool{}
		quotes := map[TSCCitation]bool{}
		var text []string
		overflow := false
		for _, row := range rows {
			if row.review {
				a.Status = "needs_review"
			}
			key := strings.ToLower(row.value)
			if values[key] {
				continue
			}
			values[key] = true
			text = append(text, row.value)
			for _, c := range row.citations {
				if !quotes[c] && len(a.Citations) >= 8 {
					overflow = true
				}
				if !quotes[c] && len(a.Citations) < 8 {
					a.Citations = append(a.Citations, c)
					quotes[c] = true
				}
			}
		}
		a.Text = strings.Join(text, "\n")
		if len(text) > 1 {
			a.Status = "conflicting"
			a.Text = "Different explicitly labelled source values require applicability/precedence review:\n" + a.Text
		}
		if overflow || len(text) > 8 || len(a.Text) > 7500 {
			a.Status = "needs_review"
			a.Text = "Many distinct labelled values were found. Review all source occurrences before selecting the applicable value."
		}
		if ValidateTSCAnswer(a, in) == nil {
			result[id] = a
		}
	}
	return result
}
func tscAdjacentFact(a, b TSCPassage) bool {
	return a.DocumentID != "" && a.DocumentID == b.DocumentID && a.Revision == b.Revision && a.Segment == b.Segment && a.End > 0 && b.Start >= a.End && b.Start-a.End <= 2
}
