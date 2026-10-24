package domain

import (
	"testing"
)

func TestAdvancesRequestCompletionDoesNotInferFinanceOrPayment(t *testing.T) {
	a, access, source, details, now := advancesFixture()
	r, err := AdvancesApply(nil, a, access, AdvancesCommand{Action: "prepare", Reason: "Prepare actual advance", Details: &details}, source, now, advancesThird, "")
	if err != nil {
		t.Fatal(err)
	}
	if p := AdvancesProgressFact(&r, source.AdvancesSourceFacts, true, now); p.State != "not_started" || p.Snapshot != nil {
		t.Fatal(p)
	}
	r, err = AdvancesApply(&r, a, access, AdvancesCommand{Version: 1, Action: "submit", Reason: "Submit actual request"}, source, now, advancesThird, "")
	if err != nil {
		t.Fatal(err)
	}
	p := AdvancesProgressFact(&r, source.AdvancesSourceFacts, true, now)
	if p.State != "completed" || p.Snapshot == nil || p.Snapshot.State != "requested" || p.Snapshot.Payment != nil || p.Snapshot.DecisionID != "" {
		t.Fatal("report invented financial decision", p)
	}
	source.Available = false
	source.Blockers = []string{"Held project"}
	if AdvancesProgressFact(&r, source.AdvancesSourceFacts, true, now).State != "completed" {
		t.Fatal("hold erased actual request")
	}
	if AdvancesCan(&r, a, access, "approve", source) == nil {
		t.Fatal("report bypassed execution hold")
	}
	p.Snapshot.Source.Blockers = append(p.Snapshot.Source.Blockers, "changed")
	*p.Snapshot.SubmittedAt = AdvancesTimestamp{}
	if r.SubmittedAt.IsZero() || len(r.Source.Blockers) != 0 {
		t.Fatal("aliased reporting snapshot")
	}
	stale := source.AdvancesSourceFacts
	stale.AcceptanceVersion++
	if p := AdvancesProgressFact(&r, stale, true, now); p.State != "blocked" || p.Current {
		t.Fatal("stale response earned progress", p)
	}
	if p := AdvancesProgressFact(&r, source.AdvancesSourceFacts, false, now); p.State != "blocked" || p.Current {
		t.Fatal("missing upstream proof", p)
	}
	r.Amount = "43.72"
	if p := AdvancesProgressFact(&r, source.AdvancesSourceFacts, true, now); p.State != "blocked" || p.Snapshot != nil {
		t.Fatal("wrong amount credited", p)
	}
}
func TestAdvancesFinanceDispositionAndPaymentRequireEvidence(t *testing.T) {
	for _, kind := range []string{"payment", "not_applicable"} {
		t.Run(kind, func(t *testing.T) {
			a, access, source, details, now := advancesFixture()
			if kind == "not_applicable" {
				details = AdvancesDetails{Kind: kind, RequestNo: "Actual no advance", RequestDate: AdvancesToday(now), Percentage: "0", Notes: "No advance agreed in PO"}
			}
			r, err := AdvancesApply(nil, a, access, AdvancesCommand{Action: "prepare", Reason: "Prepare actual disposition", Details: &details}, source, now, advancesThird, "")
			if err != nil {
				t.Fatal(err)
			}
			r, err = AdvancesApply(&r, a, access, AdvancesCommand{Version: 1, Action: "submit", Reason: "Submit actual disposition"}, source, now, advancesThird, "")
			if err != nil {
				t.Fatal(err)
			}
			a.ID = advancesSecond
			r, err = AdvancesApply(&r, a, access, AdvancesCommand{Version: 2, Action: "approve", Reason: "Finance reviewed actual disposition"}, source, now, advancesThird, advancesThird)
			if err != nil {
				t.Fatal(err)
			}
			want := "completed"
			if kind == "not_applicable" {
				want = "not_applicable"
			}
			if p := AdvancesProgressFact(&r, source.AdvancesSourceFacts, true, now); p.State != want || p.Snapshot == nil {
				t.Fatal(p)
			}
			saved := r.DecisionID
			r.DecisionID = ""
			if AdvancesProgressFact(&r, source.AdvancesSourceFacts, true, now).State != "blocked" {
				t.Fatal("invented Finance decision")
			}
			r.DecisionID = saved
			if kind == "not_applicable" {
				r.Details.Notes = ""
				if AdvancesProgressFact(&r, source.AdvancesSourceFacts, true, now).State != "blocked" {
					t.Fatal("unexplained waiver")
				}
				return
			}
			r, err = AdvancesApply(&r, a, access, AdvancesCommand{Version: 3, Action: "disburse", Reason: "Record actual payment", Payment: &AdvancesPayment{Voucher: "Actual UTR", Date: AdvancesToday(now)}}, source, now, advancesThird, "")
			if err != nil {
				t.Fatal(err)
			}
			p := AdvancesProgressFact(&r, source.AdvancesSourceFacts, true, now)
			if p.State != "completed" || p.Snapshot.Payment == nil {
				t.Fatal(p)
			}
			p.Snapshot.Payment.Voucher = "changed"
			if r.Payment.Voucher != "Actual UTR" {
				t.Fatal("aliased payment")
			}
			r.Payment = nil
			if AdvancesProgressFact(&r, source.AdvancesSourceFacts, true, now).State != "blocked" {
				t.Fatal("fabricated payment")
			}
		})
	}
}
