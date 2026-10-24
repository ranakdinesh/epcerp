package domain

import "testing"

func TestRegisterCompletion(t *testing.T) {
	for _, test := range []struct {
		name    string
		records []RegisterState
		done    bool
	}{
		{"empty", nil, false}, {"draft", []RegisterState{{State: "draft", HasItems: true}}, false},
		{"finalised", []RegisterState{{State: "completed", HasItems: true}}, true},
		{"multiple", []RegisterState{{State: "completed", HasItems: true}, {State: "completed", HasItems: true}}, true},
		{"reopened", []RegisterState{{State: "completed", HasItems: true}, {State: "draft", HasItems: true}}, false},
		{"no items", []RegisterState{{State: "completed", HasItems: false}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := RegisterCompletion(test.records)
			if (r.Percent == nil) != (len(test.records) == 0) {
				t.Fatal("missing differs from zero", r)
			}
			if test.done {
				if (r.Percent == nil || *r.Percent != 4.1) || len(r.Completed) != 1 || r.Next != "PUR-02" {
					t.Fatalf("%+v", r)
				}
			} else if (r.Percent != nil && *r.Percent != 0) || len(r.Completed) != 0 || r.Next != "PUR-01" {
				t.Fatalf("%+v", r)
			}
		})
	}
}

func TestRegisterIndentCompletion(t *testing.T) {
	for _, done := range []bool{false, true} {
		r := RegisterCompletion([]RegisterState{{State: "completed", HasItems: true, IndentComplete: true}, {State: "completed", HasItems: true, IndentComplete: done}})
		want := 4.1
		next := "PUR-02"
		steps := 1
		if done {
			want = 8.2
			next = "PUR-03"
			steps = 2
		}
		if r.Percent == nil || *r.Percent != want || r.Next != next || len(r.Completed) != steps {
			t.Fatal(r)
		}
	}
}

func TestQuotationStageCreditAndPendingCounts(t *testing.T) {
	r := RegisterCompletion([]RegisterState{{State: "completed", HasItems: true, IndentComplete: true, QuotesComplete: true, Pending: 1, Approved: 1}})
	if r.Next != "PUR-04" || len(r.Completed) != 3 || r.Pending != 1 || r.Approved != 1 {
		t.Fatal(r)
	}
	done := RegisterCompletion([]RegisterState{{State: "completed", HasItems: true, IndentComplete: true, QuotesComplete: true, ApprovalComplete: true, Approved: 2}})
	if done.Next != "PUR-05" || len(done.Completed) != 4 || *done.Percent <= *r.Percent {
		t.Fatal(done)
	}
}

func TestFinanceClearedGroupsUnlockNextPhaseIndependentlyOfOtherVendorResponses(t *testing.T) {
	p := RegisterCompletion([]RegisterState{{State: "completed", HasItems: true, IndentComplete: true, ApprovalComplete: true}})
	if p.Next != "PUR-05" || len(p.Completed) != 3 || p.Completed[2] != 4 {
		t.Fatal(p)
	}
}
