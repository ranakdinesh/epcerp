package domain

import (
	"testing"
)

func TestIndentGroupsGroupIssueCoverageAndVendorMinimum(t *testing.T) {
	g := IndentGroupsGroup{IndentGroupsGroupInput: IndentGroupsGroupInput{ID: "one", Name: "Cables", LineIDs: []string{"l1"}, VendorIDs: []string{"a", "b", "c"}}, NameKey: "cables"}
	if e := IndentGroupsValidateGroups([]IndentGroupsGroup{g}, []string{"l1"}, []string{"a", "b", "c"}, true); e != nil {
		t.Fatal(e)
	}
	if IndentGroupsValidateGroups([]IndentGroupsGroup{g}, []string{"l1", "l2"}, []string{"a", "b", "c"}, true) == nil {
		t.Fatal("unassigned source accepted")
	}
	g.VendorIDs = []string{"a", "a", "b"}
	if IndentGroupsValidateGroups([]IndentGroupsGroup{g}, []string{"l1"}, []string{"a", "b", "c"}, true) == nil {
		t.Fatal("duplicate vendor counted")
	}
	g.VendorIDs = []string{"a", "b"}
	if IndentGroupsValidateGroups([]IndentGroupsGroup{g}, []string{"l1"}, []string{"a", "b", "c"}, true) == nil {
		t.Fatal("two vendors issued")
	}
	g.VendorIDs = []string{}
	if e := IndentGroupsValidateGroups([]IndentGroupsGroup{g}, []string{"l1"}, nil, false); e != nil {
		t.Fatal("draft grouping blocked", e)
	}
}
func TestIndentGroupsHistoricalReportRetainsExactInvitationCoverage(t *testing.T) {
	q := "1"
	r := IndentGroupsRecord{State: "created", SelectedLineIDs: []string{"l1", "l2"}, SourceLines: []IndentGroupsLine{{ID: "l1", Quantity: &q}, {ID: "l2", Quantity: &q}}, Groups: []IndentGroupsGroup{{IndentGroupsGroupInput: IndentGroupsGroupInput{ID: "g", Name: "Cables", LineIDs: []string{"l1", "l2"}, VendorIDs: []string{"v"}}, ScopeRevision: 2, Invitations: []IndentGroupsInvitation{{VendorID: "v", ScopeRevision: 1, LineIDs: []string{"l1"}}}}}}
	report := IndentGroupsReport(&r, nil, []IndentGroupsVendor{{ID: "v", Name: "Vendor"}})
	if len(report) != 2 || len(report[0].Invitations) != 1 || len(report[1].Invitations) != 0 {
		t.Fatal("old invitation covered later BO line", report)
	}
}
