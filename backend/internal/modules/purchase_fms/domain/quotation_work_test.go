package domain

import (
	"strings"
	"testing"
)

func TestQuotationTasksAreResolvedAndReturnedWithinOnlyTheirGroup(t *testing.T) {
	event := QuotationApprovalsEvent{ID: "event", ProjectID: "project", BOID: "bo", ActorID: "actor", Action: "submit", ProjectLabel: "PRJ-1"}
	attempt := QuotationApprovalsAttempt{ID: "attempt", GroupID: "group", GroupName: "Cable", SubmittedBy: "executive"}
	submitted := QuotationApprovalWork(event, attempt, []string{"finance"})
	if !submitted.ResolveGroup || submitted.ResolveAttempt || len(submitted.Create) != 2 || submitted.Create[0].Kind != "finance" || submitted.Create[1].State != "waiting" || len(submitted.Notices) != 1 {
		t.Fatal(submitted)
	}
	if !strings.Contains(submitted.Create[0].Link, "&group=group&attempt=attempt") {
		t.Fatal("Finance task lost exact group identity", submitted)
	}
	event.Action = "accounts_decision"
	attempt.State = "rejected"
	attempt.ReturnAction = "change_vendor"
	returned := QuotationApprovalWork(event, attempt, []string{"executive"})
	if !returned.ResolveAttempt || returned.ResolveGroup || returned.Return == nil || returned.Return.State != "change_vendor" || len(returned.Create) != 0 || returned.GroupID != "group" {
		t.Fatal(returned)
	}
	attempt.State = "approved"
	attempt.ReturnAction = ""
	approved := QuotationApprovalWork(event, attempt, []string{"executive"})
	if approved.Return != nil || !approved.ResolveAttempt {
		t.Fatal(approved)
	}
}
