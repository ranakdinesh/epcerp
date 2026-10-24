package domain

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type HoldsError = Error

func HoldsInvalid(f, m string) error { return &HoldsError{422, f, m} }

var HoldsUnauthorized = &HoldsError{401, "", "Invalid or expired credentials"}
var HoldsForbidden = &HoldsError{403, "", "Purchase hold permission or project scope denied"}
var HoldsMissing = &HoldsError{404, "", "Purchase hold not found"}
var HoldsStale = &HoldsError{409, "", "Purchase hold or linked material requirement changed; reload the saved version"}

var holdsUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func HoldsID(s string) (string, error) {
	s = strings.ToLower(s)
	if !holdsUuid.MatchString(s) {
		return s, HoldsInvalid("id", "Use a valid hold record identifier")
	}
	return s, nil
}

type HoldsTimestamp = time.Time
type HoldsData struct {
	Scope             string     `json:"scope"`
	CategoryCode      string     `json:"category_code"`
	DemandIDs         []string   `json:"demand_ids"`
	Reason            string     `json:"reason"`
	Evidence          string     `json:"evidence"`
	StartedAt         time.Time  `json:"started_at"`
	ExpectedReleaseAt *time.Time `json:"expected_release_at"`
	DoerID            string     `json:"doer_id"`
	ResolverID        string     `json:"resolver_id"`
	ReleasedAt        *time.Time `json:"released_at"`
	ReleaseEvidence   *string    `json:"release_evidence"`
	RecordedAt        time.Time  `json:"recorded_at"`
}
type HoldsCreate struct {
	Scope             string     `json:"scope"`
	CategoryCode      string     `json:"category_code"`
	DemandIDs         []string   `json:"demand_ids"`
	Reason            string     `json:"reason"`
	Evidence          string     `json:"evidence"`
	StartedAt         time.Time  `json:"started_at"`
	ExpectedReleaseAt *time.Time `json:"expected_release_at"`
	DoerID            string     `json:"doer_id"`
	ResolverID        string     `json:"resolver_id"`
}
type HoldsDecision struct {
	Version  int32  `json:"version"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence"`
}
type HoldsRecord struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Data      HoldsData `json:"data"`
	Version   int32     `json:"version"`
	Active    bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy *string   `json:"created_by_id"`
	UpdatedBy *string   `json:"updated_by_id"`
}
type HoldsEvent struct {
	ID        string       `json:"id"`
	SubjectID string       `json:"subject_id"`
	Action    string       `json:"action"`
	Reason    string       `json:"reason"`
	Before    *HoldsRecord `json:"before"`
	After     HoldsRecord  `json:"after"`
	CreatedAt time.Time    `json:"created_at"`
	ActorID   string       `json:"created_by_id"`
}
type HoldsActiveHold struct {
	ID         string    `json:"id"`
	Reason     string    `json:"reason"`
	DoerID     string    `json:"doer_id"`
	ResolverID string    `json:"resolver_id"`
	StartedAt  time.Time `json:"started_at"`
}
type HoldsEvaluation struct {
	Holds        []HoldsActiveHold `json:"holds"`
	HoldMinutes  float64           `json:"hold_minutes"`
	FinishImpact string            `json:"finish_impact"`
}

func HoldsText(field, s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > 2000 {
		return s, HoldsInvalid(field, "Provide a value within 2000 characters")
	}
	return s, nil
}
func (c HoldsCreate) Validate(now time.Time) (HoldsCreate, error) {
	var e error
	c.Reason, e = HoldsText("reason", c.Reason)
	if e != nil {
		return c, e
	}
	c.Evidence, e = HoldsText("evidence", c.Evidence)
	if e != nil {
		return c, e
	}
	if c.StartedAt.IsZero() || c.StartedAt.After(now) || c.StartedAt.Year() < 1 || c.StartedAt.Year() > 9999 {
		return c, HoldsInvalid("started_at", "Use a valid hold start that is not in the future")
	}
	if c.ExpectedReleaseAt != nil && (c.ExpectedReleaseAt.Before(c.StartedAt) || c.ExpectedReleaseAt.Year() > 9999) {
		return c, HoldsInvalid("expected_release_at", "Expected release must not precede the hold start")
	}
	c.CategoryCode = strings.TrimSpace(c.CategoryCode)
	if utf8.RuneCountInString(c.CategoryCode) > 40 {
		return c, HoldsInvalid("category_code", "Category code exceeds 40 characters")
	}
	if c.Scope != "project" && c.Scope != "category" && c.Scope != "items" {
		return c, HoldsInvalid("scope", "Choose project, category or items")
	}
	if (c.Scope == "category") != (c.CategoryCode != "") {
		return c, HoldsInvalid("category_code", "Category is required only for category holds")
	}
	if c.Scope == "items" && len(c.DemandIDs) == 0 || c.Scope != "items" && len(c.DemandIDs) > 0 || len(c.DemandIDs) > 200 {
		return c, HoldsInvalid("demand_ids", "Select up to 200 requirements only for item holds")
	}
	seen := map[string]bool{}
	for i, id := range c.DemandIDs {
		id, e = HoldsID(id)
		if e != nil {
			return c, e
		}
		if seen[id] {
			return c, HoldsInvalid("demand_ids", "Select distinct requirements")
		}
		seen[id] = true
		c.DemandIDs[i] = id
	}
	c.DoerID, e = HoldsID(c.DoerID)
	if e != nil {
		return c, HoldsInvalid("doer_id", "Select an active doer")
	}
	c.ResolverID, e = HoldsID(c.ResolverID)
	if e != nil {
		return c, HoldsInvalid("resolver_id", "Select an active resolver")
	}
	return c, nil
}
func HoldsEvaluate(records []HoldsRecord, demand, category string, now time.Time) HoldsEvaluation {
	out := HoldsEvaluation{Holds: []HoldsActiveHold{}, FinishImpact: "Unassessed"}
	type span struct{ a, b time.Time }
	spans := []span{}
	for _, r := range records {
		d := r.Data
		related := d.Scope == "project" || d.Scope == "category" && d.CategoryCode == category
		if d.Scope == "items" {
			for _, id := range d.DemandIDs {
				if id == demand {
					related = true
				}
			}
		}
		if !related {
			continue
		}
		if r.Active && !d.StartedAt.After(now) {
			out.Holds = append(out.Holds, HoldsActiveHold{r.ID, d.Reason, d.DoerID, d.ResolverID, d.StartedAt})
		}
		end := now
		if d.ReleasedAt != nil && d.ReleasedAt.Before(end) {
			end = *d.ReleasedAt
		}
		if end.After(d.StartedAt) {
			spans = append(spans, span{d.StartedAt, end})
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].a.Before(spans[j].a) })
	merged := []span{}
	for _, s := range spans {
		if len(merged) > 0 && !s.a.After(merged[len(merged)-1].b) {
			if s.b.After(merged[len(merged)-1].b) {
				merged[len(merged)-1].b = s.b
			}
		} else {
			merged = append(merged, s)
		}
	}
	for _, s := range merged {
		out.HoldMinutes += s.b.Sub(s.a).Minutes()
	}
	out.HoldMinutes = math.Round(out.HoldMinutes*100) / 100
	return out
}
