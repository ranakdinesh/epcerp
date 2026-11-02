package domain

import (
	"fmt"
	"strings"
	"testing"
)

func factInput(ids []string, passages ...TSCPassage) TSCInput {
	in := TSCInput{Passages: passages}
	for _, id := range ids {
		in.Questions = append(in.Questions, TSCQuestion{ID: id})
	}
	return in
}
func checkFactEvidence(t *testing.T, in TSCInput, out map[string]TSCAnswer) {
	t.Helper()
	for id, a := range out {
		if a.Accepted {
			t.Fatalf("%s was automatically accepted", id)
		}
		if err := ValidateTSCAnswer(a, in); err != nil {
			t.Fatalf("%s has invalid citations: %v", id, err)
		}
	}
}
func TestFactsReadAlignedTableWrappedValuesAndUnknownRowBoundaries(t *testing.T) {
	row := func(label, value string) string { return fmt.Sprintf("%-36s%s\n", label, value) }
	text := row("Name of the Work", "Synthetic lighting installation") + row("", "including operation and maintenance") + row("Unrelated address", "Do not attach this to the work name") + row("Completion period", "180 days excluding monsoon") + row("", "and 12 months maintenance after completion") + row("Validity of the tender", "120 days from bid opening")
	in := factInput([]string{"particulars.q02", "particulars.q07", "particulars.q14"}, TSCPassage{ID: "table", Kind: "table", Text: text})
	out := CollectTSCCandidates(in)
	checkFactEvidence(t, in, out)
	if out["particulars.q02"].Text != "Synthetic lighting installation including operation and maintenance" {
		t.Fatalf("wrapped work title or field boundary lost: %q", out["particulars.q02"].Text)
	}
	if out["particulars.q07"].Text != "180 days excluding monsoon and 12 months maintenance after completion" {
		t.Fatal("phases or exception lost", out["particulars.q07"])
	}
	if out["particulars.q14"].Text != "120 days from bid opening" {
		t.Fatal("validity not read")
	}
}
func TestFactsReadMarkdownTabsSerialNumbersAndMultilineLabels(t *testing.T) {
	in := factInput([]string{"particulars.q01", "particulars.q02", "particulars.q07", "particulars.q09"},
		TSCPassage{ID: "md", Text: "| 1 | **Name of Department** | Synthetic Public Works Board |\n| 2 | Completion period | 90 days |"},
		TSCPassage{ID: "tab", Text: "3. Name of work\tSynthetic substation upgrade"},
		TSCPassage{ID: "wrapped", Text: "Last date for\nbid submission:\n25/12/2026 at 15:00 hours"})
	out := CollectTSCCandidates(in)
	checkFactEvidence(t, in, out)
	if len(out) != 4 || out["particulars.q02"].Text != "Synthetic substation upgrade" || !strings.Contains(out["particulars.q09"].Text, "15:00") {
		t.Fatalf("structured fields missing: %+v", out)
	}
}
func TestFactsKeepTaxBasisAndFeeDistinctFromEMDWithoutDecidingExemptions(t *testing.T) {
	in := factInput([]string{"particulars.q04", "particulars.q05", "tender_fee.q05", "emd.q05", "emd.q01", "tender_fee.q01"}, TSCPassage{ID: "amounts", Text: "Estimated cost (including GST): INR 118000\nEstimated cost excluding GST: INR 100000\nTender Fee: INR 2000 plus GST; pay online; exemption subject to clause 6\nEarnest Money Deposit (EMD): INR 5000; bank guarantee as specified in clause 7"})
	out := CollectTSCCandidates(in)
	checkFactEvidence(t, in, out)
	if out["particulars.q04"].Text != "INR 118000" || out["particulars.q05"].Text != "INR 100000" {
		t.Fatal("tax bases confused", out)
	}
	if out["tender_fee.q05"].Status != "needs_review" || !strings.Contains(out["tender_fee.q05"].Text, "clause 6") {
		t.Fatal("fee condition lost")
	}
	if out["emd.q05"].Status != "needs_review" || !strings.Contains(out["emd.q05"].Text, "INR 5000") || strings.Contains(out["emd.q05"].Text, "2000") {
		t.Fatal("EMD confused with fee")
	}
	if _, ok := out["emd.q01"]; ok {
		t.Fatal("exemption decision invented")
	}
	ambiguous := factInput([]string{"particulars.q04", "particulars.q05"}, TSCPassage{ID: "ambiguous", Text: "Estimated cost: INR 100000\nEstimated cost: INR 118000 including GST; base amount excluding GST INR 100000"})
	if len(CollectTSCCandidates(ambiguous)) != 0 {
		t.Fatal("tax basis inferred or compound amounts split without review")
	}
}
func TestFactsUseBoundedExactRowQuotesInsideLongPassages(t *testing.T) {
	text := strings.Repeat("Unrelated text that must not become a citation.\n", 65) + "Employer: Synthetic Water Board\n" + strings.Repeat("Other unrelated text.\n", 30)
	in := factInput([]string{"particulars.q01"}, TSCPassage{ID: "long", Text: text})
	out := CollectTSCCandidates(in)
	checkFactEvidence(t, in, out)
	a, ok := out["particulars.q01"]
	if !ok || len(a.Citations) != 1 || a.Citations[0].Quote != "Employer: Synthetic Water Board" {
		t.Fatal("valid fact lost to whole-passage citation limit", a)
	}
}
func TestFactsJoinOnlyAdjacentSameRevisionTableBlocksAndHeadingParagraphs(t *testing.T) {
	a := TSCPassage{ID: "left", DocumentID: "notice", Revision: 2, Segment: 4, Kind: "table", Start: 0, End: 100, Text: fmt.Sprintf("%-36s%s\n", "Name of work", "Synthetic water pumping")}
	b := TSCPassage{ID: "right", DocumentID: "notice", Revision: 2, Segment: 4, Kind: "table", Start: 100, End: 180, Text: fmt.Sprintf("%-36s%s\n", "", "and associated electrical works")}
	in := factInput([]string{"particulars.q02"}, a, b)
	out := CollectTSCCandidates(in)
	checkFactEvidence(t, in, out)
	if out["particulars.q02"].Text != "Synthetic water pumping and associated electrical works" || len(out["particulars.q02"].Citations) != 2 {
		t.Fatal("adjacent table continuation lost", out)
	}
	for _, mutate := range []func(*TSCPassage){func(p *TSCPassage) { p.DocumentID = "another" }, func(p *TSCPassage) { p.Revision++ }, func(p *TSCPassage) { p.Segment++ }, func(p *TSCPassage) { p.Start += 20 }, func(p *TSCPassage) { p.Alternative = true }} {
		other := b
		mutate(&other)
		out = CollectTSCCandidates(factInput([]string{"particulars.q02"}, a, other))
		if strings.Contains(out["particulars.q02"].Text, "associated") {
			t.Fatal("unrelated source continuation joined")
		}
	}
	heading := TSCPassage{ID: "h", DocumentID: "notice", Kind: "heading", Segment: 5, Start: 0, End: 25, Text: "42. COMPLETION PERIOD :"}
	paragraph := TSCPassage{ID: "p", DocumentID: "notice", Kind: "paragraph", Segment: 5, Start: 25, End: 150, Text: "The main work shall finish within 180 days, excluding the declared monsoon period."}
	in = factInput([]string{"particulars.q07"}, heading, paragraph)
	out = CollectTSCCandidates(in)
	checkFactEvidence(t, in, out)
	if out["particulars.q07"].Status != "needs_review" || len(out["particulars.q07"].Citations) != 2 {
		t.Fatal("headed clause not offered for review", out)
	}
}
func TestFactsRejectLowConfidenceTemplatesAndUnrelatedDatesAndNeverDecideCompanyEligibility(t *testing.T) {
	confidence := 69.0
	in := factInput([]string{"particulars.q01", "particulars.q02", "particulars.q07", "particulars.q08", "particulars.q09", "particulars.q16", "financial.q01", "conclusion.q01"},
		TSCPassage{ID: "low", Text: "Employer: Unsupported authority", Confidence: &confidence},
		TSCPassage{ID: "template", Text: "Name of work: _____"},
		TSCPassage{ID: "copied", Filename: "Completed TSC.docx", Text: "Employer: Copied authority"},
		TSCPassage{ID: "alt", Text: "Employer: Alternative authority", Alternative: true},
		TSCPassage{ID: "suppressed", Text: "Employer: Repeated header", Suppressed: true},
		TSCPassage{ID: "other", Text: "The employer will announce the name of work later.\nCompletion period:\nBid validity: 120 days\nPre-bid queries due date: 02/12/2026\nDate of bid opening: 12/12/2026\nContract type: Open competitive tender\nAnnual turnover: INR 1000000\nAre we eligible: Yes"})
	if out := CollectTSCCandidates(in); len(out) != 0 {
		t.Fatalf("unsupported facts or decisions invented: %+v", out)
	}
}
func TestFactsPreserveAmendedValuesAndClauseReviewStatus(t *testing.T) {
	in := factInput([]string{"particulars.q07", "financial.q02", "security.q03", "payment.q01", "financial.q01"},
		TSCPassage{ID: "original", Text: "Completion period: 90 days excluding monsoon"},
		TSCPassage{ID: "amended", DocumentType: "corrigendum", Text: "Completion period: 120 days from site handover"},
		TSCPassage{ID: "clauses", Text: "Average annual turnover requirement: INR 10000000 over the last three completed financial years\nDefect liability period: 24 months from certified completion\nPayment terms: 80% after delivery; balance after commissioning"})
	out := CollectTSCCandidates(in)
	checkFactEvidence(t, in, out)
	if out["particulars.q07"].Status != "conflicting" || len(out["particulars.q07"].Citations) != 2 {
		t.Fatal("amendment silently preferred")
	}
	for _, id := range []string{"financial.q02", "security.q03", "payment.q01"} {
		if out[id].Status != "needs_review" || out[id].Text == "" {
			t.Fatalf("clause %s was approved or lost", id)
		}
	}
	if _, ok := out["financial.q01"]; ok {
		t.Fatal("company turnover matching invented")
	}
}

func TestFactsReadWrappedSubmissionDeadlineWithoutUsingQueryOrOpeningDates(t *testing.T) {
	row := func(label, value string) string { return fmt.Sprintf("%-36s%s\n", label, value) }
	text := row("Name of work", "Synthetic lighting works") + row("Date of closing of Pre-Bid Queries", "01/12/2026 12:00") + row("http://synthetic.example/tenders/xyz", "") + row("Date of closing of e-Tender for", "") + row("", "20/12/2026 15:00") + row("submission of Bid.", "") + row("Date and time of opening of bids", "21/12/2026 16:00")
	in := factInput([]string{"particulars.q09", "particulars.q08"}, TSCPassage{ID: "dates", Kind: "table", Text: text})
	out := CollectTSCCandidates(in)
	checkFactEvidence(t, in, out)
	if out["particulars.q09"].Text != "20/12/2026 15:00" {
		t.Fatalf("deadline not captured: %+v", out)
	}
	if _, ok := out["particulars.q08"]; ok {
		t.Fatal("query deadline became pre-bid meeting date")
	}
}

func TestRepeatedValuesCannotCrowdAmendmentOutOfCitationBudget(t *testing.T) {
	var passages []TSCPassage
	for i := 0; i < 12; i++ {
		passages = append(passages, TSCPassage{ID: fmt.Sprint(i), Text: "Completion period: 90 days"})
	}
	passages = append(passages, TSCPassage{ID: "amendment", DocumentType: "corrigendum", Text: "Completion period: 120 days excluding monsoon"})
	in := factInput([]string{"particulars.q07"}, passages...)
	out := CollectTSCCandidates(in)
	checkFactEvidence(t, in, out)
	a := out["particulars.q07"]
	if a.Status != "conflicting" || len(a.Citations) != 2 || a.Citations[1].SourceID != "amendment" {
		t.Fatal("repetition hid amended evidence", a)
	}
}

func TestDurationAndDateCandidatesRequireAnActualQuantityAndDate(t *testing.T) {
	ids := []string{"particulars.q07", "particulars.q09"}
	unresolved := factInput(ids, TSCPassage{ID: "pending", Text: "Completion period: Days will be confirmed later\nTender due date: may be postponed"})
	if out := CollectTSCCandidates(unresolved); len(out) != 0 {
		t.Fatalf("unit or month word alone became a fact: %+v", out)
	}
	stated := factInput(ids, TSCPassage{ID: "stated", Text: "Completion period: six (6) calendar months excluding monsoon\nTender due date: 25 December 2026 at 15:00"})
	out := CollectTSCCandidates(stated)
	checkFactEvidence(t, stated, out)
	if len(out) != 2 {
		t.Fatal("written quantity or month date lost", out)
	}
}

func TestDirectSourceFactsAreNotCompactedIntoAnUnqualifiedAmount(t *testing.T) {
	a := TSCAnswer{QuestionID: "particulars.q05", Status: "answered", Origin: "labelled_source_candidate", Text: "INR 100000 plus the separately payable statutory charges"}
	if got := ConciseTSCAnswer(a); got.Text != a.Text {
		t.Fatal("source qualifier was discarded", got.Text)
	}
}
