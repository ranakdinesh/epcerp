package domain

import (
	"math"
	"math/big"
	"regexp"
	"slices"
	"strings"
	"time"
)

type ProgressTimestamp = time.Time
type ProgressError = Error

func ProgressInvalid(field, message string) error { return &ProgressError{422, field, message} }

var ProgressUnauthorized = &ProgressError{401, "", "Authentication required"}
var ProgressForbidden = &ProgressError{403, "", "Current purchase progress permission and project scope required"}
var progressUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ProgressID(v string) error {
	if !progressUuid.MatchString(v) {
		return ProgressInvalid("id", "Use a valid project identifier")
	}
	return nil
}

// Evidence is provided only by the transaction-bound source port. Coordination
// records are separate and cannot supply a completed/not-applicable state.
type ProgressEvidence struct {
	Number         int
	State, Details string
}
type ProgressCoordination struct {
	Number                                                             int
	State, PlannedDate, BlockReason, DoerID, ResolverID, ResolutionDue string
}
type ProgressPackage struct {
	ID, DemandID, Quantity string
	Evidence               []ProgressEvidence
	Coordination           []ProgressCoordination
	SourceBlockers         []string
}
type ProgressLine struct{ ID, DemandID, Quantity string }
type ProgressBO struct {
	ID, Name, State string
	Lines           []ProgressLine
	Grouped         []ProgressEvidence
	Packages        []ProgressPackage
}
type ProgressSource struct {
	ProjectID, ProjectStart, ProjectEnd string
	BOs                                 []ProgressBO
	Blockers                            []string
}
type ProgressStage struct {
	Number       int      `json:"number"`
	Title        string   `json:"title"`
	Weight       float64  `json:"weight"`
	Contribution float64  `json:"contribution_percent"`
	State        string   `json:"state"`
	Details      []string `json:"details"`
	Due          string   `json:"due_date"`
	Signal       string   `json:"signal"`
	MissingDates bool     `json:"missing_dates"`
	Overdue      int      `json:"days_overdue"`
	Blockers     []string `json:"blockers"`
}
type ProgressSummary struct {
	ID         string          `json:"bo_id"`
	Name       string          `json:"name"`
	Progress   float64         `json:"progress_percent"`
	Signal     string          `json:"signal"`
	Completion string          `json:"completion_date"`
	Issues     []string        `json:"issues"`
	Coverage   bool            `json:"scope_covered"`
	Stages     []ProgressStage `json:"steps"`
}
type ProgressReport struct {
	ProjectID    string            `json:"project_id"`
	AsOf         string            `json:"as_of"`
	ProjectStart string            `json:"project_start"`
	ProjectEnd   string            `json:"project_end"`
	Progress     *float64          `json:"progress_percent"`
	Packages     []ProgressSummary `json:"packages"`
	Blockers     []string          `json:"source_blockers"`
}

var progressWeights = [20]int{25, 25, 30, 10, 20, 25, 15, 50, 20, 10, 20, 240, 20, 10, 10, 10, 50, 10, 5, 5}
var progressTitles = [20]string{"BO item list", "Purchase indent", "Receive vendor quotations", "Price comparison", "Vendor onboarding & LOI", "Technical documents", "Submit to project team", "Client technical approval", "Purchase order release", "Vendor PO acceptance", "Advance payment request", "Material inspection", "Inspection report / release", "Quality certificates", "Client dispatch clearance", "Material dispatch", "Receipt at site", "GRN / warranty", "Project engineer confirmation", "Accounts acceptance"}

const progressTotalWeight = 610

func progressDone(state string) bool { return state == "completed" || state == "not_applicable" }
func progressPercent(units int) float64 {
	return float64((units*10000+progressTotalWeight/2)/progressTotalWeight) / 100
}
func progressDate(v string) (time.Time, error) {
	t, e := time.Parse("2006-01-02", v)
	if e != nil || t.Year() < 1 || t.Format("2006-01-02") != v {
		return t, ProgressInvalid("date", "Source contains an invalid ISO calendar date")
	}
	return t, nil
}

var progressQty = regexp.MustCompile(`^[0-9]{1,13}(\.[0-9]{1,3})?$`)

func progressQuantity(v string) (*big.Int, error) {
	if !progressQty.MatchString(v) {
		return nil, ProgressInvalid("quantity", "Source quantity must be a positive exact decimal with at most three places")
	}
	p := strings.Split(v, ".")
	digits := p[0] + "000"
	if len(p) == 2 {
		digits = p[0] + p[1] + strings.Repeat("0", 3-len(p[1]))
	}
	n, ok := new(big.Int).SetString(digits, 10)
	if !ok || n.Sign() <= 0 {
		return nil, ProgressInvalid("quantity", "Source quantity must be positive")
	}
	return n, nil
}
func progressSignal(state, due string, today time.Time) string {
	if progressDone(state) {
		return "complete"
	}
	if due != "" {
		d, _ := progressDate(due)
		if d.Before(today) {
			return "delayed"
		}
	}
	if state == "blocked" {
		return "blocked"
	}
	if due == "" {
		return "unplanned"
	}
	d, _ := progressDate(due)
	if d.Sub(today) <= 48*time.Hour {
		return "due_soon"
	}
	return "on_track"
}
func ProgressBuild(src ProgressSource, now ProgressTimestamp) (ProgressReport, error) {
	todayString := now.In(time.FixedZone("India", 19800)).Format("2006-01-02")
	today, e := progressDate(todayString)
	if e != nil {
		return ProgressReport{}, e
	}
	out := ProgressReport{ProjectID: src.ProjectID, AsOf: todayString, ProjectStart: src.ProjectStart, ProjectEnd: src.ProjectEnd, Packages: []ProgressSummary{}, Blockers: append([]string{}, src.Blockers...)}
	for _, d := range []string{src.ProjectStart, src.ProjectEnd} {
		if d != "" {
			if _, e := progressDate(d); e != nil {
				return ProgressReport{}, e
			}
		}
	}
	seen := map[string]bool{}
	sum := 0.0
	for _, bo := range src.BOs {
		if bo.ID == "" || seen[bo.ID] {
			return ProgressReport{}, ProgressInvalid("bo", "Use unique current BO identifiers")
		}
		seen[bo.ID] = true
		r, e := progressBuildBO(bo, src, today)
		if e != nil {
			return ProgressReport{}, e
		}
		out.Packages = append(out.Packages, r)
		sum += r.Progress
	}
	if len(out.Packages) > 0 {
		v := math.RoundToEven(sum/float64(len(out.Packages))*100) / 100
		out.Progress = &v
	}
	return out, nil
}
func progressBuildBO(bo ProgressBO, src ProgressSource, today time.Time) (ProgressSummary, error) {
	out := ProgressSummary{ID: bo.ID, Name: bo.Name, Issues: []string{}, Stages: []ProgressStage{}}
	demands := map[string]*big.Int{}
	lineIDs := map[string]bool{}
	coverage := len(bo.Lines) > 0
	for _, l := range bo.Lines {
		if l.ID == "" || lineIDs[l.ID] {
			return out, ProgressInvalid("line", "Use unique current BO lines")
		}
		lineIDs[l.ID] = true
		var n *big.Int
		if l.Quantity == "" && bo.State != "completed" {
			coverage = false
		} else {
			var e error
			n, e = progressQuantity(l.Quantity)
			if e != nil {
				return out, e
			}
		}
		if l.DemandID == "" {
			coverage = false
			continue
		}
		if _, ok := demands[l.DemandID]; ok {
			return out, ProgressInvalid("demand", "BO lines must map to distinct demand identities")
		}
		demands[l.DemandID] = n
	}
	pkgIDs := map[string]bool{}
	quantities := map[string]*big.Int{}
	facts := make([]map[int]ProgressEvidence, len(bo.Packages))
	plans := make([]map[int]ProgressCoordination, len(bo.Packages))
	for i, p := range bo.Packages {
		if p.ID == "" || pkgIDs[p.ID] {
			return out, ProgressInvalid("package", "Use unique current work packages")
		}
		pkgIDs[p.ID] = true
		if _, ok := demands[p.DemandID]; !ok {
			return out, ProgressInvalid("demand", "Work package must belong to a current BO demand")
		}
		n, e := progressQuantity(p.Quantity)
		if e != nil {
			return out, e
		}
		if quantities[p.DemandID] == nil {
			quantities[p.DemandID] = new(big.Int)
		}
		quantities[p.DemandID].Add(quantities[p.DemandID], n)
		facts[i] = map[int]ProgressEvidence{}
		plans[i] = map[int]ProgressCoordination{}
		for _, f := range p.Evidence {
			if f.Number < 2 || f.Number > 20 || facts[i][f.Number].Number != 0 || !slices.Contains([]string{"not_started", "in_progress", "blocked", "completed", "not_applicable"}, f.State) || f.State == "not_applicable" && (f.Number != 11 || strings.TrimSpace(f.Details) == "") {
				return out, ProgressInvalid("evidence", "Use unique owned stage facts; N/A is only the advance disposition")
			}
			facts[i][f.Number] = f
		}
		for _, c := range p.Coordination {
			if c.Number < 2 || c.Number > 20 || plans[i][c.Number].Number != 0 || !slices.Contains([]string{"source_linked", "not_started", "preparing", "blocked"}, c.State) {
				return out, ProgressInvalid("coordination", "Coordination cannot declare specialist completion")
			}
			for _, d := range []string{c.PlannedDate, c.ResolutionDue} {
				if d != "" {
					if _, e := progressDate(d); e != nil {
						return out, e
					}
				}
			}
			plans[i][c.Number] = c
			if c.Number == 20 && c.PlannedDate > out.Completion {
				out.Completion = c.PlannedDate
			}
		}
	}
	for id, required := range demands {
		if required == nil || quantities[id] == nil || quantities[id].Cmp(required) != 0 {
			coverage = false
		}
	}
	out.Coverage = coverage
	grouped := map[int]ProgressEvidence{}
	for _, g := range bo.Grouped {
		if g.Number < 2 || g.Number > 4 || grouped[g.Number].Number != 0 || !slices.Contains([]string{"not_started", "in_progress", "blocked", "completed"}, g.State) {
			return out, ProgressInvalid("grouped", "Current grouped evidence may supply only stages 2–4")
		}
		grouped[g.Number] = g
	}
	earned := 0
	complete := true
	for number := 1; number <= 20; number++ {
		stage := ProgressStage{Number: number, Title: progressTitles[number-1], Weight: float64(progressWeights[number-1]) / 10, Contribution: progressPercent(progressWeights[number-1]), State: "not_started", Details: []string{}, Blockers: []string{}, MissingDates: !coverage}
		if number == 1 {
			if bo.State == "completed" && len(bo.Lines) > 0 {
				stage.State = "completed"
			} else {
				stage.State = "in_progress"
			}
		} else {
			all := coverage && len(bo.Packages) > 0
			anyWork, blocked, allNA := false, false, true
			for i := range bo.Packages {
				f, exists := facts[i][number]
				c, planned := plans[i][number]
				finished := exists && progressDone(f.State)
				all = all && finished
				allNA = allNA && f.State == "not_applicable"
				blocked = blocked || f.State == "blocked" || (!finished && c.State == "blocked")
				anyWork = anyWork || (exists && f.State != "not_started") || c.State == "preparing"
				if f.Details != "" {
					stage.Details = append(stage.Details, f.Details)
					if f.State == "blocked" {
						stage.Blockers = append(stage.Blockers, f.Details)
					}
				}
				if !finished && (len(bo.Packages[i].SourceBlockers) > 0 || len(src.Blockers) > 0) {
					blocked = true
					stage.Blockers = append(stage.Blockers, bo.Packages[i].SourceBlockers...)
					stage.Blockers = append(stage.Blockers, src.Blockers...)
				}
				if !finished && c.State == "blocked" && c.BlockReason != "" {
					stage.Blockers = append(stage.Blockers, c.BlockReason)
				}
				if !planned || c.PlannedDate == "" {
					stage.MissingDates = true
				}
				outstanding := !finished
				if g, ok := grouped[number]; ok {
					outstanding = !progressDone(g.State)
				}
				if outstanding && c.PlannedDate != "" && (stage.Due == "" || c.PlannedDate < stage.Due) {
					stage.Due = c.PlannedDate
				}
			}
			if all {
				stage.State = "completed"
				if number == 11 && allNA {
					stage.State = "not_applicable"
				}
			} else if blocked {
				stage.State = "blocked"
			} else if anyWork {
				stage.State = "in_progress"
			}
		}
		if g, ok := grouped[number]; ok {
			stage.State = g.State
			stage.Details = append(stage.Details, g.Details)
		}
		if progressDone(stage.State) {
			stage.Due = ""
			stage.Blockers = []string{}
			earned += progressWeights[number-1]
		} else {
			complete = false
		}
		if !progressDone(stage.State) && (len(stage.Blockers) > 0 || len(src.Blockers) > 0) {
			stage.State = "blocked"
			if number == 1 {
				stage.Blockers = append(stage.Blockers, src.Blockers...)
			}
		}
		stage.Signal = progressSignal(stage.State, stage.Due, today)
		if stage.Due != "" && !progressDone(stage.State) {
			d, _ := progressDate(stage.Due)
			if d.Before(today) {
				stage.Overdue = int(today.Sub(d) / (24 * time.Hour))
			}
		}
		out.Stages = append(out.Stages, stage)
	}
	out.Progress = progressPercent(earned)
	if out.Completion == "" || out.Stages[19].MissingDates {
		out.Issues = append(out.Issues, "Purchase completion date not fully planned (Step 20)")
	}
	if src.ProjectStart == "" || src.ProjectEnd == "" {
		out.Issues = append(out.Issues, "Project start/end dates missing")
	}
	if out.Completion != "" && src.ProjectEnd != "" && out.Completion > src.ProjectEnd {
		out.Issues = append(out.Issues, "Purchase completion is after project end")
	}
	if out.Completion != "" && src.ProjectStart != "" && out.Completion < src.ProjectStart {
		out.Issues = append(out.Issues, "Purchase completion is before project start")
	}
	if src.ProjectStart != "" && src.ProjectEnd != "" && src.ProjectStart > src.ProjectEnd {
		out.Issues = append(out.Issues, "Project date range is invalid")
	}
	if !coverage {
		out.Issues = append(out.Issues, "Current purchase packages do not cover all BO lines and quantities")
	}
	out.Signal = "on_track"
	switch {
	case complete:
		out.Signal = "complete"
	case progressHasSignal(out.Stages, "delayed") || src.ProjectEnd != "" && src.ProjectEnd < today.Format("2006-01-02") || out.Completion != "" && out.Completion < today.Format("2006-01-02"):
		out.Signal = "delayed"
	case progressHasSignal(out.Stages, "blocked"):
		out.Signal = "blocked"
	case progressHasSignal(out.Stages, "due_soon"):
		out.Signal = "due_soon"
	case len(out.Issues) > 0:
		out.Signal = "attention"
	default:
		for _, s := range out.Stages {
			if !progressDone(s.State) && s.MissingDates {
				out.Signal = "unplanned"
				break
			}
		}
	}
	return out, nil
}
func progressHasSignal(stages []ProgressStage, v string) bool {
	for _, s := range stages {
		if s.Signal == v {
			return true
		}
	}
	return false
}
