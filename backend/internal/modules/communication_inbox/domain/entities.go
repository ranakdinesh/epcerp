package domain

import (
	"regexp"
	"strings"
	"time"
)

type Timestamp = time.Time

type Error struct {
	Status         int
	Field, Message string
}

func (e *Error) Error() string  { return e.Message }
func Invalid(f, m string) error { return &Error{422, f, m} }

var Unauthorized = &Error{401, "", "Invalid or expired credentials"}
var Forbidden = &Error{403, "", "Personal inbox permission denied"}
var Missing = &Error{404, "", "Inbox notice not found"}
var Stale = &Error{409, "", "Notice changed; reload before updating the receipt"}
var NoDirector = &Error{409, "", "An effective Director must be designated before the financial decision"}
var SourceConflict = &Error{409, "", "The financial decision event does not match a valid notice source"}
var pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ID(s string) (string, error) {
	s = strings.ToLower(s)
	if !pattern.MatchString(s) {
		return s, Invalid("id", "Use a valid notice identifier")
	}
	return s, nil
}

type Actor struct {
	ID                string
	Active, Superuser bool
	Permissions       []string
}

func (a Actor) Can(code string) bool {
	if !a.Active || a.ID == "" {
		return false
	}
	if a.Superuser {
		return true
	}
	for _, p := range a.Permissions {
		if p == code {
			return true
		}
	}
	return false
}

type Director struct {
	ID, UserID string
	Version    int32
}
type Source struct {
	EventID, EntityID, ProjectID, ActorID, Module, EntityType, Action, ScopeKind, Visibility string
	Version                                                                                  int64
	At                                                                                       time.Time
}

func (s Source) Validate() error {
	for _, id := range []string{s.EventID, s.EntityID, s.ProjectID, s.ActorID} {
		if _, e := ID(id); e != nil {
			return SourceConflict
		}
	}
	if (s.Module != "finance_approvals" && s.Module != "finance_accounts_acceptance") || s.EntityType != "financial_decision" || s.ScopeKind != "project" || s.Visibility != "restricted" || s.Version < 1 || s.At.IsZero() {
		return SourceConflict
	}
	switch s.Action {
	case "approved", "rejected", "returned", "cancelled":
		return nil
	}
	return SourceConflict
}

type Notice struct {
	SourceActionID  string     `json:"source_action_id,omitempty"`
	Kind            string     `json:"notice_kind,omitempty"`
	Title           string     `json:"title,omitempty"`
	Link            string     `json:"link,omitempty"`
	ID              string     `json:"id"`
	EventID         string     `json:"source_event_id"`
	RecipientID     string     `json:"recipient_id"`
	DirectorID      string     `json:"director_designation_id"`
	DirectorVersion int32      `json:"director_version"`
	EntityID        string     `json:"decision_id"`
	ProjectID       string     `json:"project_id"`
	SourceVersion   int64      `json:"source_version"`
	Action          string     `json:"decision_action"`
	Version         int32      `json:"version"`
	Active          bool       `json:"is_active"`
	ReadAt          *time.Time `json:"read_at"`
	AcknowledgedAt  *time.Time `json:"acknowledged_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	CreatedBy       *string    `json:"created_by_id"`
	UpdatedBy       *string    `json:"updated_by_id"`
}

func New(id string, s Source, d Director) (Notice, error) {
	if e := s.Validate(); e != nil {
		return Notice{}, e
	}
	for _, v := range []string{id, d.ID, d.UserID} {
		if _, e := ID(v); e != nil {
			return Notice{}, e
		}
	}
	if d.Version < 1 {
		return Notice{}, NoDirector
	}
	actor := s.ActorID
	return Notice{ID: id, EventID: s.EventID, RecipientID: d.UserID, DirectorID: d.ID, DirectorVersion: d.Version, EntityID: s.EntityID, ProjectID: s.ProjectID, SourceVersion: s.Version, Action: s.Action, Version: 1, Active: true, CreatedAt: s.At, UpdatedAt: s.At, CreatedBy: &actor, UpdatedBy: &actor}, nil
}
func (n Notice) Matches(s Source) bool {
	return n.EventID == s.EventID && n.EntityID == s.EntityID && n.ProjectID == s.ProjectID && n.SourceVersion == s.Version && n.Action == s.Action && n.CreatedBy != nil && *n.CreatedBy == s.ActorID
}
func Receipt(n Notice, actor string, version int32, action string, now time.Time) (Notice, bool, error) {
	if n.RecipientID != actor {
		return Notice{}, false, Missing
	}
	if version != n.Version {
		return Notice{}, false, Stale
	}
	if action != "read" && action != "acknowledge" {
		return Notice{}, false, Invalid("action", "Choose read or acknowledge")
	}
	if now.IsZero() || now.Before(n.UpdatedAt) {
		return Notice{}, false, Invalid("time", "Receipt time unavailable")
	}
	if action == "read" && n.ReadAt != nil || action == "acknowledge" && n.AcknowledgedAt != nil {
		return n, false, nil
	}
	now = now.UTC().Truncate(time.Microsecond)
	if n.ReadAt == nil {
		n.ReadAt = &now
	}
	if action == "acknowledge" {
		n.AcknowledgedAt = &now
	}
	n.Version++
	n.UpdatedAt = now
	n.UpdatedBy = &actor
	return n, true, nil
}

type Event struct {
	ID      string    `json:"id"`
	Action  string    `json:"action"`
	Before  *Notice   `json:"before"`
	After   Notice    `json:"after"`
	ActorID string    `json:"created_by_id"`
	At      time.Time `json:"created_at"`
}
type Query struct {
	Status     string
	Page, Size int32
}

func (q Query) Validate() (Query, error) {
	if q.Page < 1 || q.Page > 100000 || q.Size != 20 && q.Size != 50 && q.Size != 100 {
		return q, Invalid("page", "Use bounded inbox pages of 20, 50 or 100 rows")
	}
	switch q.Status {
	case "", "unread", "read", "acknowledged":
		return q, nil
	}
	return q, Invalid("status", "Unknown inbox status")
}

type Listing struct {
	Items    []Notice `json:"items"`
	Total    int64    `json:"total"`
	Page     int32    `json:"page"`
	Size     int32    `json:"size"`
	CanWrite bool     `json:"can_write"`
}
type Workspace struct {
	Notice   Notice  `json:"notice"`
	History  []Event `json:"history"`
	CanWrite bool    `json:"can_write"`
}
