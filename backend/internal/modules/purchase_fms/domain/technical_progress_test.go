package domain

import (
	"testing"
)

func technicalProgressFixture() (TechnicalCase, TechnicalSource, []TechnicalFile) {
	_, _, source, _, _ := technicalTechnicalFixture()
	file := TechnicalFile{ID: technicalUid(100), CaseID: technicalUid(7), Purpose: "document", Size: 100, Active: true, MediaType: "application/pdf"}
	doc := TechnicalDocument{ID: technicalUid(30), Kind: "ga", Required: true, Status: "received", FileID: file.ID, Active: true}
	c := TechnicalCase{ID: technicalUid(7), ProjectID: source.ProjectID, PackageID: source.PackageID, Source: source, Version: 2, Revision: 1, State: "draft", Active: true, Documents: []TechnicalDocument{doc}}
	return c, source, []TechnicalFile{file}
}
func TestTechnicalTechnicalReceiptAndSubmissionAreIndependentOfClientClearance(t *testing.T) {
	c, current, files := technicalProgressFixture()
	p := TechnicalProgressFacts(&c, current, files, true)
	if !p.Current || p.Snapshot == nil || p.Steps[0].State != "completed" || p.Steps[1].State != "not_started" || p.Steps[2].State != "not_started" {
		t.Fatal(p)
	}
	p.Snapshot.Documents[0].Status = "changed"
	if c.Documents[0].Status != "received" {
		t.Fatal("aliased private reporting snapshot")
	}
	files = append(files, TechnicalFile{ID: technicalUid(200), CaseID: c.ID, Purpose: "transmittal", Size: 100, Active: true, MediaType: "application/pdf"})
	c.State = "submitted"
	c.Submission = &TechnicalSubmission{Revision: c.Revision, Reference: "Actual TR", FileID: technicalUid(200), Documents: append([]TechnicalDocument{}, c.Documents...), SubmittedBy: technicalUid(1)}
	p = TechnicalProgressFacts(&c, current, files, true)
	if p.Steps[1].State != "completed" || p.Steps[2].State != "in_progress" {
		t.Fatal(p)
	}
	if TechnicalReadyWithEvidence(c, files) {
		t.Fatal("reporting advanced PO authority")
	}
	c.Revision++
	p = TechnicalProgressFacts(&c, current, files, true)
	if p.Steps[1].State == "completed" {
		t.Fatal("stale submission earned completion", p)
	}
}
func TestTechnicalTechnicalProgressHoldPreservationAndStaleParent(t *testing.T) {
	c, current, files := technicalProgressFixture()
	current.Available = false
	current.Blockers = []string{"Project on hold"}
	p := TechnicalProgressFacts(&c, current, files, true)
	if !p.Current || p.Steps[0].State != "completed" {
		t.Fatal("held receipt erased", p)
	}
	if TechnicalCurrentSourceGuard(c, current) == nil {
		t.Fatal("hold became execution authority")
	}
	p = TechnicalProgressFacts(&c, current, files, false)
	if p.Current || p.Steps[0].State != "blocked" {
		t.Fatal("unproven upstream credited", p)
	}
	current.Version++
	p = TechnicalProgressFacts(&c, current, files, true)
	if p.Current || p.Steps[0].State != "blocked" {
		t.Fatal("changed LOI version credited", p)
	}
}
func TestTechnicalTechnicalProgressFilesAndRejections(t *testing.T) {
	c, current, files := technicalProgressFixture()
	files[0].Active = false
	p := TechnicalProgressFacts(&c, current, files, true)
	if p.Steps[0].State == "completed" {
		t.Fatal("inactive evidence credited")
	}
	c.State = "revision_required"
	c.TeamReview = &TechnicalTeamReview{Remarks: "Correct specification"}
	p = TechnicalProgressFacts(&c, current, files, true)
	if p.Steps[1].State != "blocked" || p.Steps[1].Details != "Correct specification" {
		t.Fatal(p)
	}
	c.State = "rejected"
	c.Approval = &TechnicalApproval{Remarks: "Rejected actual kit"}
	p = TechnicalProgressFacts(&c, current, files, true)
	if p.Steps[2].State != "blocked" || p.Steps[2].Details != "Rejected actual kit" {
		t.Fatal(p)
	}
	c.Active = false
	p = TechnicalProgressFacts(&c, current, files, true)
	if p.CaseID != "" || p.Steps[0].State != "not_started" {
		t.Fatal("inactive case advertised")
	}
}
