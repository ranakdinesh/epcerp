package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const inspectionsProject = "11111111-1111-4111-8111-111111111111"
const inspectionsPkg = "22222222-2222-4222-8222-222222222222"
const inspectionsActorID = "33333333-3333-4333-8333-333333333333"
const inspectionsIdentity = "44444444-4444-4444-8444-444444444444"

func inspectionsFlag(v bool) *bool { return &v }
func inspectionsSource() InspectionsSource {
	return InspectionsSource{InspectionsSourceFacts: InspectionsSourceFacts{ProjectID: inspectionsProject, PackageID: inspectionsPkg, OrderID: inspectionsIdentity, OrderVersion: 2, Number: "Actual PO", Date: "2026-10-10", SupplierID: inspectionsIdentity, SupplierName: "Actual supplier", Description: "Actual cable", Quantity: "10", UOM: "metre", AcceptanceID: inspectionsIdentity, AcceptanceVersion: 1, TechnicalID: inspectionsIdentity, TechnicalVersion: 12, TechnicalRevision: 1, QAPReference: "Approved QAP reference", QAPDate: "2026-10-09", QAPFileID: inspectionsIdentity, SignedApprovalFileID: inspectionsIdentity}, Available: true, Blockers: []string{}}
}
func inspectionsCompleted() InspectionsInspectionInput {
	return InspectionsInspectionInput{Status: "cleared", CallReference: "Actual inspection call", CallDate: "2026-10-10", ScheduledDate: "2026-10-11", ActualDate: "2026-10-11", Agency: "client_consultant", Inspector: "Actual inspector", Factory: "Actual works", Findings: "Recorded measured values satisfy the approved QAP", Points: []InspectionsWitnessInput{{Code: "WP-01", Title: "Actual QAP dimensional test", Mandatory: inspectionsFlag(true), Cleared: inspectionsFlag(true), Remarks: "Recorded measurement"}}}
}
func inspectionsNow() time.Time   { return time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC) }
func inspectionsOwner() Principal { return Principal{ID: inspectionsActorID, Active: true} }
func inspectionsCaps() InspectionsAccess {
	return InspectionsAccess{Read: true, Write: true, Work: true, Scoped: true}
}
func TestInspectionsInspectionTruthfulCompletionRevisionAndDefensiveCopies(t *testing.T) {
	s := inspectionsSource()
	in := inspectionsCompleted()
	q := InspectionsCommand{Action: "record", Reason: "Actual witnessed inspection", Inspection: &in}
	r, e := InspectionsApply(nil, inspectionsOwner(), inspectionsCaps(), q, s, inspectionsNow(), inspectionsIdentity)
	if e != nil || r.Inspection.Status != "cleared" || r.Revision != 1 || r.Version != 1 {
		t.Fatal(e, r)
	}
	in.Points[0].Title = "Changed caller input"
	if r.Inspection.Points[0].Title == in.Points[0].Title {
		t.Fatal("Caller points alias saved record")
	}
	fact, e := InspectionsClearedFact(&r, s)
	if e != nil {
		t.Fatal(e)
	}
	fact.Inspection.Points[0].Title = "Changed consumer"
	fact.Source.Blockers = append(fact.Source.Blockers, "forged")
	if r.Inspection.Points[0].Title == fact.Inspection.Points[0].Title || len(r.Source.Blockers) > 0 {
		t.Fatal("Consumer changed saved evidence")
	}
	q.Version = 1
	if _, e = InspectionsApply(&r, inspectionsOwner(), inspectionsCaps(), q, s, inspectionsNow(), inspectionsIdentity); !errors.Is(e, InspectionsStale) {
		t.Fatal("Final decision overwritten", e)
	}
	reopened, e := InspectionsApply(&r, inspectionsOwner(), inspectionsCaps(), InspectionsCommand{Version: 1, Action: "revise", Reason: "Actual rectification needs reinspection"}, s, inspectionsNow(), inspectionsIdentity)
	if e != nil || reopened.Revision != 2 || reopened.Inspection.Status != "not_scheduled" || len(reopened.Inspection.Points) != 0 || r.Inspection.Status != "cleared" {
		t.Fatal("Effective clearance or history overwritten", e)
	}
	if _, e = InspectionsClearedFact(&reopened, s); e == nil {
		t.Fatal("Reopened inspection published as cleared")
	}
	stale := s
	stale.TechnicalVersion++
	if _, e = InspectionsClearedFact(&r, stale); !errors.Is(e, InspectionsStale) {
		t.Fatal("Superseded QAP still cleared", e)
	}
	if _, e = InspectionsApply(&reopened, inspectionsOwner(), inspectionsCaps(), InspectionsCommand{Version: 2, Action: "record", Reason: "Stale source", Inspection: &in}, stale, inspectionsNow(), inspectionsIdentity); !errors.Is(e, InspectionsStale) {
		t.Fatal("Old source overwritten without revision", e)
	}
}
func TestInspectionsInspectionCompletionRequiresRealVisitAndMandatoryPoints(t *testing.T) {
	tests := []struct {
		name   string
		change func(*InspectionsInspectionInput)
	}{
		{"no actual visit", func(q *InspectionsInspectionInput) { q.ActualDate = "" }},
		{"planned future visit", func(q *InspectionsInspectionInput) { q.ActualDate = "2026-10-12" }},
		{"missing works", func(q *InspectionsInspectionInput) { q.Factory = " " }},
		{"missing inspector", func(q *InspectionsInspectionInput) { q.Inspector = "" }},
		{"missing agency", func(q *InspectionsInspectionInput) { q.Agency = "" }},
		{"invented agency", func(q *InspectionsInspectionInput) { q.Agency = "AAI automatic" }},
		{"missing findings", func(q *InspectionsInspectionInput) { q.Findings = "" }},
		{"open mandatory point", func(q *InspectionsInspectionInput) { q.Points[0].Cleared = inspectionsFlag(false) }},
		{"omitted cleared flag", func(q *InspectionsInspectionInput) { q.Points[0].Cleared = nil }},
		{"omitted mandatory flag", func(q *InspectionsInspectionInput) { q.Points[0].Mandatory = nil }},
		{"empty checklist unexplained", func(q *InspectionsInspectionInput) { q.Points = nil }},
		{"duplicate codes", func(q *InspectionsInspectionInput) {
			q.Points = append(q.Points, q.Points[0])
			q.Points[1].Code = "wp-01"
		}},
		{"empty criteria", func(q *InspectionsInspectionInput) { q.Points[0].Title = "" }},
		{"call after actual", func(q *InspectionsInspectionInput) { q.CallDate = "2026-10-12" }},
		{"call before PO", func(q *InspectionsInspectionInput) { q.CallDate = "2026-10-09" }},
		{"oversized notes", func(q *InspectionsInspectionInput) { q.Findings = strings.Repeat("x", 8001) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := inspectionsCompleted()
			tc.change(&in)
			if _, e := InspectionsApply(nil, inspectionsOwner(), inspectionsCaps(), InspectionsCommand{Action: "record", Reason: "Recorded decision", Inspection: &in}, inspectionsSource(), inspectionsNow(), inspectionsIdentity); e == nil {
				t.Fatal("Unsupported clearance accepted")
			}
		})
	}
	in := inspectionsCompleted()
	in.Points = nil
	in.NoPointsReason = "Approved QAP specifies no witness checkpoints for this material"
	if _, e := InspectionsApply(nil, inspectionsOwner(), inspectionsCaps(), InspectionsCommand{Action: "record", Reason: "Actual inspection and explicit QAP disposition", Inspection: &in}, inspectionsSource(), inspectionsNow(), inspectionsIdentity); e != nil {
		t.Fatal("No-points QAP rejected despite actual reason", e)
	}
	in = inspectionsCompleted()
	in.Points[0].Mandatory = inspectionsFlag(false)
	in.Points[0].Cleared = inspectionsFlag(false)
	if _, e := InspectionsApply(nil, inspectionsOwner(), inspectionsCaps(), InspectionsCommand{Action: "record", Reason: "Actual clearance with optional unperformed point", Inspection: &in}, inspectionsSource(), inspectionsNow(), inspectionsIdentity); e != nil {
		t.Fatal("Optional point treated as mandatory", e)
	}
}
func TestInspectionsInspectionSchedulingPreservesPythonDateAlternativeAndFuturePlan(t *testing.T) {
	for _, in := range []InspectionsInspectionInput{{Status: "scheduled", ScheduledDate: "2026-10-20", Factory: "Actual works"}, {Status: "scheduled", ActualDate: "2026-10-11", Factory: "Actual works"}, {Status: "not_scheduled"}, {Status: "call_received", CallReference: "Actual call", CallDate: "2026-10-11"}} {
		if _, e := InspectionsApply(nil, inspectionsOwner(), inspectionsCaps(), InspectionsCommand{Action: "record", Reason: "Actual scheduling", Inspection: &in}, inspectionsSource(), inspectionsNow(), inspectionsIdentity); e != nil {
			t.Fatal("Valid source scheduling rejected", e)
		}
	}
	in := inspectionsCompleted()
	in.ScheduledDate = "2026-10-20"
	if _, e := InspectionsApply(nil, inspectionsOwner(), inspectionsCaps(), InspectionsCommand{Action: "record", Reason: "Actual earlier visit", Inspection: &in}, inspectionsSource(), inspectionsNow(), inspectionsIdentity); e != nil {
		t.Fatal("Actual earlier visit must remain truthful", e)
	}
	for _, in := range []InspectionsInspectionInput{{Status: "scheduled", Factory: "works"}, {Status: "scheduled", ScheduledDate: "2026-10-20"}, {Status: "rejected", ActualDate: "2026-10-11"}} {
		if _, e := InspectionsApply(nil, inspectionsOwner(), inspectionsCaps(), InspectionsCommand{Action: "record", Reason: "r", Inspection: &in}, inspectionsSource(), inspectionsNow(), inspectionsIdentity); e == nil {
			t.Fatal("Incomplete visit accepted")
		}
	}
}
func TestInspectionsInspectionScopeSourceVersionAndStrictCommands(t *testing.T) {
	in := inspectionsCompleted()
	q := InspectionsCommand{Action: "record", Reason: "Actual inspection", Inspection: &in}
	for _, c := range []InspectionsAccess{{Read: true, Write: true}, {Read: true, Work: true}, {Write: true, Work: true}} {
		if _, e := InspectionsApply(nil, inspectionsOwner(), c, q, inspectionsSource(), inspectionsNow(), inspectionsIdentity); !errors.Is(e, InspectionsForbidden) {
			t.Fatal("Unassigned write permitted", e)
		}
	}
	s := inspectionsSource()
	s.Available = false
	if _, e := InspectionsApply(nil, inspectionsOwner(), inspectionsCaps(), q, s, inspectionsNow(), inspectionsIdentity); e == nil {
		t.Fatal("Held source inspected")
	}
	if _, e := InspectionsApply(nil, inspectionsOwner(), inspectionsCaps(), InspectionsCommand{Version: 1, Action: "record", Reason: "r", Inspection: &in}, inspectionsSource(), inspectionsNow(), inspectionsIdentity); !errors.Is(e, InspectionsStale) {
		t.Fatal("Wrong initial version accepted")
	}
	for _, q := range []InspectionsCommand{{Action: "record", Reason: "r"}, {Action: "revise", Reason: "r", Inspection: &in}, {Action: "approve", Reason: "r"}, {Action: "record", Inspection: &in}} {
		if _, e := InspectionsValidate(q); e == nil {
			t.Fatal("Invalid command accepted")
		}
	}
}
