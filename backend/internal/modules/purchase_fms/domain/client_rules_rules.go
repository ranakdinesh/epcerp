package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type ClientRulesError = Error

func ClientRulesInvalid(field, message string) error { return &ClientRulesError{422, field, message} }

var ClientRulesForbidden = &ClientRulesError{403, "", "Client purchase policy permission denied"}
var ClientRulesUnauthorized = &ClientRulesError{401, "", "Invalid or expired credentials"}
var ClientRulesMissing = &ClientRulesError{404, "", "Client purchase rule not found"}
var ClientRulesStale = &ClientRulesError{409, "", "Client purchase rule changed; reload before saving"}

var clientRulesUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ClientRulesID(s string) (string, error) {
	s = strings.ToLower(s)
	if !clientRulesUuid.MatchString(s) {
		return s, ClientRulesInvalid("id", "Use a valid record identifier")
	}
	return s, nil
}

type ClientRulesData struct {
	ProjectID      *string  `json:"project_id"`
	ScopeCode      string   `json:"scope_code"`
	Mode           string   `json:"mode"`
	Allowed        []string `json:"allowed"`
	Excluded       []string `json:"excluded"`
	EffectiveFrom  string   `json:"effective_from"`
	EffectiveUntil *string  `json:"effective_until"`
	Evidence       string   `json:"evidence"`
}
type ClientRulesRecord struct {
	ID        string          `json:"id"`
	ClientID  string          `json:"client_id"`
	Data      ClientRulesData `json:"data"`
	Active    bool            `json:"is_active"`
	Version   int32           `json:"version"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	CreatedBy *string         `json:"created_by_id"`
	UpdatedBy *string         `json:"updated_by_id"`
}
type ClientRulesSave struct {
	Data    ClientRulesData `json:"data"`
	Version int32           `json:"version"`
	Active  bool            `json:"is_active"`
	Reason  string          `json:"reason"`
}
type ClientRulesEvent struct {
	ID        string             `json:"id"`
	SubjectID string             `json:"subject_id"`
	Action    string             `json:"action"`
	Reason    string             `json:"reason"`
	Before    *ClientRulesRecord `json:"before"`
	After     ClientRulesRecord  `json:"after"`
	CreatedAt time.Time          `json:"created_at"`
	ActorID   string             `json:"created_by_id"`
}
type ClientRulesRuleEvidence struct {
	ID       string `json:"id"`
	Version  int32  `json:"version"`
	Evidence string `json:"evidence"`
}
type ClientRulesVerdict struct {
	Status  string                    `json:"status"`
	Reasons []string                  `json:"reasons"`
	Rules   []ClientRulesRuleEvidence `json:"rules"`
}
type ClientRulesEvaluation struct{ ClientID, ProjectID, Category, Family, Configuration, Manufacturer, Today string }

func clientRulesDate(s string) bool {
	v, e := time.Parse("2006-01-02", s)
	return e == nil && v.Year() > 0 && v.Year() <= 9999 && v.Format("2006-01-02") == s
}
func (s ClientRulesSave) Validate() (ClientRulesSave, error) {
	s.Reason = strings.TrimSpace(s.Reason)
	if s.Reason == "" || utf8.RuneCountInString(s.Reason) > 2000 {
		return s, ClientRulesInvalid("reason", "Provide a reason within 2000 characters")
	}
	if s.Version < 1 {
		return s, ClientRulesInvalid("version", "Use the current rule version")
	}
	d := &s.Data
	d.ScopeCode = strings.TrimSpace(d.ScopeCode)
	d.Evidence = strings.TrimSpace(d.Evidence)
	if d.ScopeCode == "" || utf8.RuneCountInString(d.ScopeCode) > 40 {
		return s, ClientRulesInvalid("scope_code", "Choose a category, configuration or *")
	}
	if d.Mode != "unrestricted" && d.Mode != "approved_only" {
		return s, ClientRulesInvalid("mode", "Choose unrestricted or approved_only")
	}
	if d.Evidence == "" || utf8.RuneCountInString(d.Evidence) > 2000 {
		return s, ClientRulesInvalid("evidence", "Provide client policy evidence within 2000 characters")
	}
	if !clientRulesDate(d.EffectiveFrom) {
		return s, ClientRulesInvalid("effective_from", "Use a valid ISO date")
	}
	if d.EffectiveUntil != nil && (!clientRulesDate(*d.EffectiveUntil) || *d.EffectiveUntil < d.EffectiveFrom) {
		return s, ClientRulesInvalid("effective_until", "Rule expiry must not precede its start")
	}
	if d.ProjectID != nil {
		v, e := ClientRulesID(*d.ProjectID)
		if e != nil {
			return s, ClientRulesInvalid("project_id", "Choose a valid project")
		}
		d.ProjectID = &v
	}
	if len(d.Allowed) > 200 || len(d.Excluded) > 200 {
		return s, ClientRulesInvalid("allowed", "Use at most 200 manufacturers in each list")
	}
	if d.Mode == "approved_only" && len(d.Allowed) == 0 {
		return s, ClientRulesInvalid("allowed", "An approved-only rule requires manufacturers")
	}
	seen := map[string]bool{}
	for i, id := range d.Allowed {
		v, e := ClientRulesID(id)
		if e != nil {
			return s, ClientRulesInvalid("allowed", "Choose valid manufacturers")
		}
		if seen[v] {
			return s, ClientRulesInvalid("allowed", "Duplicate manufacturer")
		}
		d.Allowed[i] = v
		seen[v] = true
	}
	excluded := map[string]bool{}
	for i, id := range d.Excluded {
		v, e := ClientRulesID(id)
		if e != nil {
			return s, ClientRulesInvalid("excluded", "Choose valid manufacturers")
		}
		if seen[v] {
			return s, ClientRulesInvalid("excluded", "A manufacturer cannot be both allowed and excluded")
		}
		if excluded[v] {
			return s, ClientRulesInvalid("excluded", "Duplicate manufacturer")
		}
		d.Excluded[i] = v
		excluded[v] = true
	}
	if d.Allowed == nil {
		d.Allowed = []string{}
	}
	if d.Excluded == nil {
		d.Excluded = []string{}
	}
	return s, nil
}
func clientRulesContains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func ClientRulesEvaluate(records []ClientRulesRecord, q ClientRulesEvaluation) ClientRulesVerdict {
	// Only effective, applicable client instructions restrict a make. No matching
	// policy means no restriction; it is not a missing mandatory approval. Keep
	// Rules empty in that case so downstream snapshots do not invent a review.
	out := ClientRulesVerdict{Status: "eligible", Reasons: []string{}, Rules: []ClientRulesRuleEvidence{}}
	for _, r := range records {
		d := r.Data
		if !r.Active || (d.ProjectID != nil && *d.ProjectID != q.ProjectID) || d.EffectiveFrom > q.Today || (d.EffectiveUntil != nil && *d.EffectiveUntil < q.Today) {
			continue
		}
		if d.ScopeCode != "*" && d.ScopeCode != q.Category && d.ScopeCode != q.Family && d.ScopeCode != q.Configuration {
			continue
		}
		out.Rules = append(out.Rules, ClientRulesRuleEvidence{r.ID, r.Version, d.Evidence})
		if clientRulesContains(d.Excluded, q.Manufacturer) {
			out.Reasons = append(out.Reasons, "Manufacturer excluded by "+d.ScopeCode+" policy")
		}
		if d.Mode == "approved_only" && !clientRulesContains(d.Allowed, q.Manufacturer) {
			out.Reasons = append(out.Reasons, "Manufacturer outside approved list for "+d.ScopeCode)
		}
	}
	if len(out.Reasons) > 0 {
		out.Status = "blocked"
	}
	return out
}
