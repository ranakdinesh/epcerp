package domain

import (
	"testing"
)

func TestWorkflowLiteralPackageSearch(t *testing.T) {
	f, e := (WorkflowPageFilter{Search: `CAB_10%\end`}).Validate()
	if e != nil || f.Pattern() != `%CAB\_10\%\\end%` {
		t.Fatalf("literal search: %q %v", f.Pattern(), e)
	}
	for _, f := range []WorkflowPageFilter{{Page: -1}, {Size: 1000}, {Sort: "source"}, {Direction: "injected"}} {
		if _, e := f.Validate(); e == nil {
			t.Fatal("accepted invalid paging")
		}
	}
}
