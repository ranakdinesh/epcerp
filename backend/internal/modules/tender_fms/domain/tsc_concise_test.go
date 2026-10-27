package domain

import "testing"

func TestCompactTSCAnswer(t *testing.T) {
	for _, tc := range []struct{ question, text, want string }{
		{"particulars.q05", "The tender value without GST is INR 3,74,62,212.00. This is stated in the document as estimated cost (S2). The estimated bid value is INR 37462212 (S4).", "INR 3,74,62,212.00"},
		{"particulars.q05", "Between INR 100 and INR 200", "Between INR 100 and INR 200"},
		{"particulars.q05", "Approximately INR 100 crore", "Approximately INR 100 crore"},
		{"particulars.q06", "Reverse Auction (RA) is applicable to the first 50% of technically qualified bidders, arranged in order of prices from lowest to highest. If the number is odd, round up. For example, 7 bids means 4 bidders.", "Yes; the first 50% of technically qualified bidders, ranked by price (lowest first)"},
		{"particulars.q07", "90 days from LOA. Except monsoon delays. This is stated in the document (S1).", "90 days from LOA; Except monsoon delays"},
		{"particulars.q07", "90 days\nfrom LOA; subject to site handover", "90 days from LOA; subject to site handover"},
	} {
		if got := CompactTSCText(tc.question, tc.text); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.question, got, tc.want)
		}
	}
	for _, a := range []TSCAnswer{{Status: "answered", Text: "Reviewed. Text.", Accepted: true}, {Status: "answered", Text: "Manual. Text.", Origin: "manual"}, {Status: "conflicting", Text: "INR 100 or INR 200"}} {
		if got := ConciseTSCAnswer(a); got.Text != a.Text {
			t.Fatal("Reviewed/manual/conflicting answer was rewritten")
		}
	}
}
