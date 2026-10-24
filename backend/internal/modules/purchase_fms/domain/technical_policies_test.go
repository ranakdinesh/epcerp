package domain

import (
	"fmt"
	"testing"
	"time"
)

func technicalUid(i int) string  { return fmt.Sprintf("%08x-1111-4111-8111-111111111111", i) }
func technicalFlag(v bool) *bool { return &v }
func technicalTechnicalFixture() (Principal, TechnicalAccess, TechnicalSource, TechnicalTimestamp, []string) {
	ids := []string{}
	for i := range 6 {
		ids = append(ids, technicalUid(30+i))
	}
	return Principal{ID: technicalUid(1), Active: true}, TechnicalAccess{Read: true, Write: true, Approve: true, Scoped: true, Work: map[int]bool{6: true, 7: true, 8: true}}, TechnicalSource{CaseID: technicalUid(6), Version: 11, ProjectID: technicalUid(3), PackageID: technicalUid(4), SupplierID: technicalUid(5), LOIReference: "LOI-released", Description: "Reviewed cable", Quantity: "10", Currency: "INR", Total: "100.00", FinancialReference: "actual-approval:fingerprint", ReleasedDate: "2026-10-11", Available: true}, time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC), ids
}
func TestTechnicalFullKitSubmissionIndependentReviewAndActualClientEvidence(t *testing.T) {
	actor, caps, source, now, ids := technicalTechnicalFixture()
	var current *TechnicalCase
	files := []TechnicalFile{}
	run := func(q TechnicalCommand) {
		t.Helper()
		q.Reason = "Actual technical evidence"
		if current != nil {
			q.Version = current.Version
		}
		next, e := TechnicalApply(current, actor, caps, q, source, files, now, technicalUid(7), ids)
		if e != nil {
			t.Fatal(q.Action, e)
		}
		current = &next
	}
	run(TechnicalCommand{Action: "configure"})
	for _, d := range current.Documents {
		if d.Status != "pending" || d.FileID != "" || current.Approval != nil {
			t.Fatal("example approvals became operational data")
		}
	}
	if _, e := TechnicalApply(current, actor, caps, TechnicalCommand{Version: current.Version, Action: "submit", Reason: "Try missing evidence", Submission: &TechnicalSubmissionInput{Reference: "TR", Date: "2026-10-11", TargetDate: "2026-10-20", Authority: "Actual client", FileID: technicalUid(200)}}, source, files, now, technicalUid(7), ids); e == nil {
		t.Fatal("missing full kit submitted")
	}
	for i, d := range append([]TechnicalDocument{}, current.Documents...) {
		if !d.Required {
			continue
		}
		file := TechnicalFile{ID: technicalUid(100 + i), CaseID: technicalUid(7), Purpose: "document", Active: true, Size: 100, MediaType: "application/pdf"}
		files = append(files, file)
		run(TechnicalCommand{Action: "document", Document: &TechnicalDocumentInput{ID: d.ID, Kind: d.Kind, Title: d.Title, Category: d.Category, Standard: d.Standard, Required: technicalFlag(true), Active: technicalFlag(true), Status: "received", Revision: "Rev 1", ReceivedDate: "2026-10-11", FileID: file.ID}})
	}
	files = append(files, TechnicalFile{ID: technicalUid(200), CaseID: technicalUid(7), Purpose: "transmittal", Active: true, Size: 100, MediaType: "application/pdf"})
	run(TechnicalCommand{Action: "submit", Submission: &TechnicalSubmissionInput{Reference: "TR-real", Date: "2026-10-11", TargetDate: "2026-10-20", Authority: "Client consultant", FileID: technicalUid(200)}})
	captured := TechnicalCopy(*current)
	if _, e := TechnicalApply(current, actor, caps, TechnicalCommand{Version: current.Version, Action: "team_review", Reason: "Try self review", Approve: technicalFlag(true), Remarks: "Approve"}, source, files, now, technicalUid(7), ids); e == nil {
		t.Fatal("self-review permitted")
	}
	actor.ID = technicalUid(2)
	run(TechnicalCommand{Action: "team_review", Approve: technicalFlag(true), Remarks: "Project team checked exact submitted revision"})
	actor.ID = technicalUid(1)
	files = append(files, TechnicalFile{ID: technicalUid(201), CaseID: technicalUid(7), Purpose: "client_approval", Active: true, Size: 100, MediaType: "application/pdf"})
	approval := &TechnicalApprovalInput{Status: "approved_with_comments", Reference: "Client-letter", Date: "2026-10-11", Name: "Recorded external approver", Designation: "Client engineer", FileID: technicalUid(201), Remarks: "Close annotated conditions"}
	run(TechnicalCommand{Action: "client_approval", Approval: approval, QAPApproved: technicalFlag(true), ConditionsClosed: technicalFlag(false)})
	if TechnicalReadyWithEvidence(*current, files) {
		t.Fatal("open conditional approval authorized PO")
	}
	files = append(files, TechnicalFile{ID: technicalUid(202), CaseID: technicalUid(7), Purpose: "conditions", Active: true, Size: 100, MediaType: "application/pdf"})
	approval.ClosureFileID = technicalUid(202)
	run(TechnicalCommand{Action: "client_approval", Approval: approval, QAPApproved: technicalFlag(true), ConditionsClosed: technicalFlag(true)})
	if !TechnicalReadyWithEvidence(*current, files) {
		t.Fatal("later condition closure required a false new submission")
	}
	run(TechnicalCommand{Action: "revise"})
	if current.Submission != nil || current.TeamReview != nil || current.Approval != nil || TechnicalReady(*current) {
		t.Fatal("revision retained obsolete approval")
	}
	if captured.State != "submitted" || captured.Submission.Revision >= current.Revision {
		t.Fatal("history snapshot mutated")
	}
	run(TechnicalCommand{Action: "submit", Submission: &TechnicalSubmissionInput{Reference: "TR-corrected", Date: "2026-10-11", TargetDate: "2026-10-20", Authority: "Client consultant", FileID: technicalUid(200)}})
	actor.ID = technicalUid(2)
	run(TechnicalCommand{Action: "team_review", Approve: technicalFlag(true), Remarks: "Independent corrected package review"})
	actor.ID = technicalUid(1)
	files = append(files, TechnicalFile{ID: technicalUid(203), CaseID: technicalUid(7), Purpose: "conditions", Active: true, Size: 100, MediaType: "application/pdf"})
	approval.ClosureFileID = technicalUid(203)
	run(TechnicalCommand{Action: "client_approval", Approval: approval, QAPApproved: technicalFlag(true), ConditionsClosed: technicalFlag(true)})
	if !TechnicalReadyWithEvidence(*current, files) {
		t.Fatal("actual cleared kit not ready")
	}
	held := source
	held.Available = false
	if TechnicalCurrentReady(*current, held, files) {
		t.Fatal("held project advertised PO readiness")
	}
	changed := source
	changed.LOIReference = "Different actual released LOI"
	if TechnicalCurrentReady(*current, changed, files) {
		t.Fatal("obsolete LOI advertised PO readiness")
	}
	if !TechnicalCurrentReady(*current, source, files) {
		t.Fatal("current cleared technical source not ready")
	}
	invalid := TechnicalCopy(*current)
	invalid.Documents[0].Revision = "Changed"
	if TechnicalReady(invalid) {
		t.Fatal("changed bundle accepted against old submission")
	}
	files[len(files)-1].Active = false
	if TechnicalReadyWithEvidence(*current, files) {
		t.Fatal("withdrawn closure evidence accepted")
	}
}
func TestTechnicalDocumentPurposeOwnershipRequiredFlagsAndTaskAuthority(t *testing.T) {
	a, caps, src, now, ids := technicalTechnicalFixture()
	c, e := TechnicalApply(nil, a, caps, TechnicalCommand{Action: "configure", Reason: "Create actual checklist"}, src, nil, now, technicalUid(7), ids)
	if e != nil {
		t.Fatal(e)
	}
	d := c.Documents[0]
	input := TechnicalDocumentInput{ID: d.ID, Kind: d.Kind, Title: d.Title, Category: d.Category, Standard: d.Standard, Required: technicalFlag(true), Active: technicalFlag(true), Status: "received", Revision: "Rev 1", ReceivedDate: "2026-10-11", FileID: technicalUid(100)}
	for _, file := range []TechnicalFile{{ID: technicalUid(100), CaseID: technicalUid(9), Purpose: "document", Active: true, Size: 10, MediaType: "application/pdf"}, {ID: technicalUid(100), CaseID: c.ID, Purpose: "client_approval", Active: true, Size: 10, MediaType: "application/pdf"}, {ID: technicalUid(100), CaseID: c.ID, Purpose: "document", Active: true, Size: 10, MediaType: "text/html"}} {
		if _, e := TechnicalValidateDocument(input, c, []TechnicalFile{file}, TechnicalToday(now)); e == nil {
			t.Fatal("unowned or wrong-purpose evidence accepted")
		}
	}
	qap := c.Documents[4]
	input = TechnicalDocumentInput{ID: qap.ID, Kind: qap.Kind, Title: qap.Title, Category: qap.Category, Standard: qap.Standard, Required: technicalFlag(false), Active: technicalFlag(true), Status: "pending", Revision: "Rev 0", NAReason: "Skip"}
	if _, e := TechnicalValidateDocument(input, c, nil, TechnicalToday(now)); e == nil {
		t.Fatal("mandatory QAP disabled")
	}
	caps.Work = map[int]bool{7: true}
	if _, e := TechnicalApply(&c, a, caps, TechnicalCommand{Version: c.Version, Action: "revise", Reason: "Try outside assigned task"}, src, nil, now, technicalUid(7), ids); e == nil {
		t.Fatal("task7 granted document mutation")
	}
	src.Available = false
	caps.Work = map[int]bool{6: true}
	if _, e := TechnicalApply(&c, a, caps, TechnicalCommand{Version: c.Version, Action: "document", Reason: "Try held mutation", Document: &input}, src, nil, now, technicalUid(7), ids); e == nil {
		t.Fatal("held source mutated")
	}
}

func TestTechnicalReleasedSourceRequiresRealCommercialIdentity(t *testing.T) {
	_, _, src, _, _ := technicalTechnicalFixture()
	for _, change := range []func(*TechnicalSource){
		func(s *TechnicalSource) { s.Quantity = "" },
		func(s *TechnicalSource) { s.Quantity = "0" },
		func(s *TechnicalSource) { s.Quantity = "1e4" },
		func(s *TechnicalSource) { s.Quantity = "10.00001" },
		func(s *TechnicalSource) { s.Total = "-1.00" },
		func(s *TechnicalSource) { s.Total = "100.001" },
		func(s *TechnicalSource) { s.Currency = "inr" },
		func(s *TechnicalSource) { s.FinancialReference = "" },
	} {
		invalid := src
		change(&invalid)
		if invalid.Guard() == nil {
			t.Fatal("incomplete released LOI accepted", invalid)
		}
	}
	if src.Guard() != nil {
		t.Fatal("actual released source rejected")
	}
}
