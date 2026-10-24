package domain

import (
	"testing"
	"time"
)

func progressCurrentSource() ProgressSource {
	p := ProgressPackage{ID: "p1", DemandID: "d1", Quantity: "100"}
	for n := 2; n <= 20; n++ {
		p.Evidence = append(p.Evidence, ProgressEvidence{Number: n, State: "completed"})
		p.Coordination = append(p.Coordination, ProgressCoordination{Number: n, State: "not_started", PlannedDate: "2026-10-20"})
	}
	return ProgressSource{ProjectID: "project", ProjectStart: "2026-10-01", ProjectEnd: "2026-10-30", BOs: []ProgressBO{{ID: "bo1", Name: "BO cable", State: "completed", Lines: []ProgressLine{{ID: "line1", DemandID: "d1", Quantity: "100"}}, Packages: []ProgressPackage{p}}}}
}
func progressNow() ProgressTimestamp { return time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC) }
func TestProgressActualCompleteWeightAndEmptyRegister(t *testing.T) {
	s := progressCurrentSource()
	r, e := ProgressBuild(s, progressNow())
	if e != nil || r.Progress == nil || *r.Progress != 100 || r.Packages[0].Signal != "complete" || len(r.Packages[0].Stages) != 20 {
		t.Fatal(r, e)
	}
	total := 0.0
	for _, stage := range r.Packages[0].Stages {
		total += stage.Weight
		if stage.Signal != "complete" || stage.Overdue != 0 {
			t.Fatal(stage)
		}
	}
	if total != 61 {
		t.Fatal("Python weight fidelity", total)
	}
	s.BOs = nil
	r, e = ProgressBuild(s, progressNow())
	if e != nil || r.Progress != nil || r.Packages == nil {
		t.Fatal("empty register fabricated progress", r, e)
	}
}
func TestProgressCoordinationNeverCompletesSpecialist(t *testing.T) {
	s := progressCurrentSource()
	s.BOs[0].Packages[0].Evidence = nil
	for i := range s.BOs[0].Packages[0].Coordination {
		s.BOs[0].Packages[0].Coordination[i].State = "preparing"
	}
	r, e := ProgressBuild(s, progressNow())
	if e != nil || r.Packages[0].Progress != 4.1 || r.Packages[0].Stages[8].State != "in_progress" {
		t.Fatal(r, e)
	}
	s.BOs[0].Packages[0].Coordination[0].State = "completed"
	if _, e = ProgressBuild(s, progressNow()); e == nil {
		t.Fatal("generic completion bypass")
	}
}
func TestProgressPartialAndSplitExactCoverage(t *testing.T) {
	s := progressCurrentSource()
	s.BOs[0].Packages[0].Quantity = "40"
	r, e := ProgressBuild(s, progressNow())
	if e != nil || r.Packages[0].Coverage || r.Packages[0].Progress != 4.1 {
		t.Fatal("partial demand completed BO", r, e)
	}
	p := s.BOs[0].Packages[0]
	p.ID = "p2"
	p.Quantity = "60"
	s.BOs[0].Packages = append(s.BOs[0].Packages, p)
	r, e = ProgressBuild(s, progressNow())
	if e != nil || !r.Packages[0].Coverage || r.Packages[0].Progress != 100 {
		t.Fatal("split demand coverage", r, e)
	}
	s.BOs[0].Packages[1].Quantity = "60.001"
	r, e = ProgressBuild(s, progressNow())
	if e != nil || r.Packages[0].Coverage {
		t.Fatal("overallocated quantity credited", r, e)
	}
}
func TestProgressDonePackagesNeverMakeOutstandingWorkOverdue(t *testing.T) {
	s := progressCurrentSource()
	p := s.BOs[0].Packages[0]
	p.Quantity = "50"
	s.BOs[0].Packages[0] = p
	p.ID = "p2"
	p.Evidence[7] = ProgressEvidence{Number: 9, State: "in_progress"}
	p.Coordination = append([]ProgressCoordination{}, p.Coordination...)
	p.Coordination[7].PlannedDate = "2026-10-13"
	// Copy facts: changing outstanding evidence must not alter the completed package.
	s.BOs[0].Packages[0].Evidence = append([]ProgressEvidence{}, s.BOs[0].Packages[0].Evidence...)
	s.BOs[0].Packages[0].Evidence[7] = ProgressEvidence{Number: 9, State: "completed"}
	s.BOs[0].Packages[0].Coordination = append([]ProgressCoordination{}, s.BOs[0].Packages[0].Coordination...)
	s.BOs[0].Packages[0].Coordination[7].PlannedDate = "2026-10-01"
	s.BOs[0].Packages = append(s.BOs[0].Packages, p)
	r, e := ProgressBuild(s, progressNow())
	if e != nil {
		t.Fatal(e)
	}
	stage := r.Packages[0].Stages[8]
	if stage.Due != "2026-10-13" || stage.Signal != "due_soon" || stage.Overdue != 0 {
		t.Fatal(stage)
	}
}
func TestProgressGroupedFactsCannotEraseReleasedLOI(t *testing.T) {
	s := progressCurrentSource()
	s.BOs[0].Grouped = []ProgressEvidence{{Number: 2, State: "completed"}, {Number: 3, State: "completed"}, {Number: 4, State: "completed"}}
	r, e := ProgressBuild(s, progressNow())
	if e != nil || r.Packages[0].Stages[4].State != "completed" || r.Packages[0].Progress != 100 {
		t.Fatal("Python grouped Step5 bug retained", r, e)
	}
	s.BOs[0].Grouped = append(s.BOs[0].Grouped, ProgressEvidence{Number: 5, State: "not_started"})
	if _, e = ProgressBuild(s, progressNow()); e == nil {
		t.Fatal("group overwrote owner LOI")
	}
	s = progressCurrentSource()
	s.BOs[0].Grouped = []ProgressEvidence{{Number: 3, State: "in_progress"}}
	s.BOs[0].Packages[0].Coordination[1].PlannedDate = "2026-10-10"
	r, e = ProgressBuild(s, progressNow())
	if e != nil || r.Packages[0].Stages[2].Overdue != 1 {
		t.Fatal("outstanding grouped scope lost due date", r, e)
	}
}
func TestProgressNAIsReasonedAdvanceOnly(t *testing.T) {
	s := progressCurrentSource()
	s.BOs[0].Packages[0].Evidence[9] = ProgressEvidence{Number: 11, State: "not_applicable", Details: "No advance agreed"}
	r, e := ProgressBuild(s, progressNow())
	if e != nil || r.Packages[0].Stages[10].State != "not_applicable" || r.Packages[0].Progress != 100 {
		t.Fatal(r, e)
	}
	s.BOs[0].Packages[0].Evidence[9].Details = ""
	if _, e = ProgressBuild(s, progressNow()); e == nil {
		t.Fatal("unreasoned NA")
	}
	s.BOs[0].Packages[0].Evidence[9] = ProgressEvidence{Number: 9, State: "not_applicable", Details: "Bypass"}
	if _, e = ProgressBuild(s, progressNow()); e == nil {
		t.Fatal("PO NA bypass")
	}
}
func TestProgressIndiaDateBlockerAndScheduleIssues(t *testing.T) {
	s := progressCurrentSource()
	s.ProjectStart = "2026-11-01"
	s.ProjectEnd = "2026-10-10"
	s.BOs[0].Packages[0].Evidence[10] = ProgressEvidence{Number: 12, State: "in_progress"}
	s.BOs[0].Packages[0].Coordination[10] = ProgressCoordination{Number: 12, State: "blocked", PlannedDate: "2026-10-10", BlockReason: "Witness unavailable"}
	r, e := ProgressBuild(s, time.Date(2026, 10, 10, 19, 0, 0, 0, time.UTC))
	if e != nil {
		t.Fatal(e)
	}
	p := r.Packages[0]
	stage := p.Stages[11]
	if r.AsOf != "2026-10-11" || p.Signal != "delayed" || stage.State != "blocked" || stage.Overdue != 1 || len(stage.Blockers) != 1 || len(p.Issues) != 3 {
		t.Fatal(r)
	}
}
func TestProgressInvalidSourceAndImmutableInput(t *testing.T) {
	s := progressCurrentSource()
	before := progressCurrentSource()
	r, e := ProgressBuild(s, progressNow())
	if e != nil {
		t.Fatal(e)
	}
	r.Packages[0].Stages[0].Details = append(r.Packages[0].Stages[0].Details, "changed")
	if s.BOs[0].Packages[0].Evidence[0] != before.BOs[0].Packages[0].Evidence[0] || len(s.BOs[0].Packages[0].Evidence) != len(before.BOs[0].Packages[0].Evidence) {
		t.Fatal("source mutation")
	}
	s.BOs[0].Packages[0].Quantity = "1e2"
	if _, e = ProgressBuild(s, progressNow()); e == nil {
		t.Fatal("float quantity")
	}
	s = progressCurrentSource()
	s.BOs[0].Packages[0].Coordination[0].PlannedDate = "2026-02-30"
	if _, e = ProgressBuild(s, progressNow()); e == nil {
		t.Fatal("invalid source date")
	}
	s = progressCurrentSource()
	s.BOs[0].Packages = append(s.BOs[0].Packages, s.BOs[0].Packages[0])
	if _, e = ProgressBuild(s, progressNow()); e == nil {
		t.Fatal("duplicate package credit")
	}
	s = progressCurrentSource()
	s.BOs[0].Packages[0].DemandID = "other"
	if _, e = ProgressBuild(s, progressNow()); e == nil {
		t.Fatal("foreign BO package")
	}
}
func TestProgressHugeExactSplitDoesNotOverflow(t *testing.T) {
	s := progressCurrentSource()
	s.BOs[0].Lines[0].Quantity = "9999999999999.999"
	s.BOs[0].Packages = nil
	for i := 0; i < 1000; i++ {
		p := progressCurrentSource().BOs[0].Packages[0]
		p.ID = string(rune(i + 1))
		p.Quantity = "9999999999999.999"
		s.BOs[0].Packages = append(s.BOs[0].Packages, p)
	}
	r, e := ProgressBuild(s, progressNow())
	if e != nil || r.Packages[0].Coverage {
		t.Fatal("overflow or overcoverage", e)
	}
}

func TestProgressDraftPendingQuantityIsVisibleAsIncomplete(t *testing.T) {
	s := progressCurrentSource()
	s.BOs[0].State = "draft"
	s.BOs[0].Lines[0].Quantity = ""
	r, e := ProgressBuild(s, progressNow())
	if e != nil || r.Packages[0].Coverage || r.Packages[0].Progress != 0 {
		t.Fatal("pending draft hidden or completed", r, e)
	}
	s.BOs[0].State = "completed"
	if _, e = ProgressBuild(s, progressNow()); e == nil {
		t.Fatal("corrupt published quantity accepted")
	}
}

func TestProgressSourceHoldDoesNotEraseCompletedEvidence(t *testing.T) {
	s := progressCurrentSource()
	s.Blockers = []string{"Project on hold"}
	s.BOs[0].Packages[0].Evidence[10] = ProgressEvidence{Number: 12, State: "in_progress"}
	r, e := ProgressBuild(s, progressNow())
	if e != nil {
		t.Fatal(e)
	}
	if r.Packages[0].Stages[8].State != "completed" || r.Packages[0].Stages[11].State != "blocked" || r.Packages[0].Signal != "blocked" || len(r.Blockers) != 1 {
		t.Fatal("hold erased history or was hidden", r)
	}
	r.Blockers[0] = "changed"
	if s.Blockers[0] != "Project on hold" {
		t.Fatal("aliased source blockers")
	}
}
