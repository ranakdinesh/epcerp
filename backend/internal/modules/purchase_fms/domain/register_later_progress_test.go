package domain

import "testing"

func TestLaterStageCreditNeedsFullCurrentDemandCoverage(t *testing.T) {
	lines := []ProgressLine{{DemandID: "d", Quantity: "10.000"}}
	packages := []ProgressPackage{{ID: "p", DemandID: "d", Quantity: "10.000", Evidence: []ProgressEvidence{{Number: 5, State: "completed"}, {Number: 6, State: "not_applicable"}, {Number: 7, State: "in_progress"}}}}
	if got := LaterRegisterCompletion(lines, packages); len(got) != 2 || got[0] != 5 || got[1] != 6 {
		t.Fatal(got)
	}
	packages[0].Quantity = "10.0000"
	if len(LaterRegisterCompletion(lines, packages)) != 2 {
		t.Fatal("equivalent legacy decimal lost credit")
	}
	packages[0].Quantity = "9.000"
	if len(LaterRegisterCompletion(lines, packages)) != 0 {
		t.Fatal("partial scope earned credit")
	}
	packages[0].Quantity = "10.000"
	packages[0].SourceBlockers = []string{"obsolete demand"}
	if len(LaterRegisterCompletion(lines, packages)) != 0 {
		t.Fatal("stale source earned credit")
	}
}
func TestRegisterCompletesAllTwentyStages(t *testing.T) {
	later := []int{}
	for i := 5; i <= 20; i++ {
		later = append(later, i)
	}
	r := RegisterCompletion([]RegisterState{{State: "completed", HasItems: true, IndentComplete: true, QuotesComplete: true, ApprovalComplete: true, LaterCompleted: later}})
	if r.Percent == nil || *r.Percent != 100 || len(r.Completed) != 20 {
		t.Fatal(r)
	}
	r = RegisterCompletion([]RegisterState{{State: "completed", HasItems: true, IndentComplete: true, QuotesComplete: true, ApprovalComplete: true, LaterCompleted: []int{5, 6}}})
	if r.Next != "PUR-07" || len(r.Completed) != 6 {
		t.Fatal(r)
	}
}

func TestClientPOSignoffIsAnIndependentNavigationGate(t *testing.T) {
	r := RegisterState{State: "completed", HasItems: true, IndentComplete: true, QuotesComplete: true, ApprovalComplete: true, LaterCompleted: []int{5, 6, 7, 8}}
	result := RegisterCompletion([]RegisterState{r})
	if result.Next != "PUR-PO-CLIENT" {
		t.Fatal("Skipped required client PO signoff", result)
	}
	r.LaterCompleted = append(r.LaterCompleted, 21)
	result = RegisterCompletion([]RegisterState{r})
	if result.Next != "PUR-09" {
		t.Fatal("Did not advance to PO release", result)
	}
	r.LaterCompleted = append(r.LaterCompleted, 9)
	result = RegisterCompletion([]RegisterState{r})
	if result.Next != "PUR-10" {
		t.Fatal("Did not advance to vendor acknowledgement", result)
	}
}
