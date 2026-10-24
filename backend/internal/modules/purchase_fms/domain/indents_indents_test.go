package domain

import (
	"testing"
)

func TestIndentsIndentDistinctAssignmentsAndExactAmounts(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	other := "22222222-2222-4222-8222-222222222222"
	input := IndentsSave{Version: 1, Reason: "Prepare indent", Data: IndentsInput{Currency: "INR", DoerID: id, ResolverID: other, Documents: "BO", Lines: []IndentsLine{{DemandID: id, Quantity: "1.125", Budget: "9999999999999999.99", FullKit: "Reviewed specification"}}}}
	if _, e := input.Validate(); e != nil {
		t.Fatal(e)
	}
	input.Data.ResolverID = id
	if _, e := input.Validate(); e == nil {
		t.Fatal("self resolver")
	}
	for _, s := range []string{"-1", "1e2", "1.001", "10000000000000000"} {
		if IndentsMoney(s) {
			t.Fatal(s)
		}
	}
	input.Data.ResolverID = other
	input.Data.Lines = append(input.Data.Lines, input.Data.Lines[0])
	if _, e := input.Validate(); e == nil {
		t.Fatal("duplicate demand")
	}
}
func TestIndentsIndentReleaseChecksCurrentPolicyHoldsAndRemainder(t *testing.T) {
	line := IndentsLine{Quantity: "4.001"}
	source := IndentsSource{Active: true, Quantity: "10", Policy: IndentsPolicy{Status: "eligible"}}
	if e := IndentsReleaseGuard(line, source, 5999); e != nil {
		t.Fatal(e)
	}
	if e := IndentsReleaseGuard(line, source, 6000); e == nil {
		t.Fatal("oversubscription")
	}
	source.Policy.Status = "not_reviewed"
	if e := IndentsReleaseGuard(line, source, 0); e == nil {
		t.Fatal("unreviewed policy")
	}
	source.Policy.Status = "eligible"
	source.Holds.Holds = []IndentsHold{{ID: "hold"}}
	if e := IndentsReleaseGuard(line, source, 0); e == nil {
		t.Fatal("held purchase")
	}
	source.Holds.Holds = nil
	source.Active = false
	if e := IndentsReleaseGuard(line, source, 0); e == nil {
		t.Fatal("inactive source")
	}
}
