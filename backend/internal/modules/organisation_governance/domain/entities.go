package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type Error struct {
	Status         int
	Field, Message string
}

func (e *Error) Error() string            { return e.Message }
func Invalid(field, message string) error { return &Error{422, field, message} }

var Unauthorized = &Error{401, "", "Invalid or expired credentials"}
var Forbidden = &Error{403, "", "Governance permission denied"}
var Stale = &Error{409, "", "Director designation changed; reload before saving"}
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ID(s string) (string, error) {
	s = strings.ToLower(s)
	if !uuid.MatchString(s) {
		return s, Invalid("user_id", "Choose a valid account identifier")
	}
	return s, nil
}

type Timestamp = time.Time
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

type User struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Active      bool     `json:"is_active"`
	Superuser   bool     `json:"-"`
	Permissions []string `json:"-"`
}

func (u User) Actor() Actor {
	return Actor{ID: u.ID, Active: u.Active, Superuser: u.Superuser, Permissions: u.Permissions}
}

type Designation struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Version   int32     `json:"version"`
	Active    bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy *string   `json:"created_by_id"`
	UpdatedBy *string   `json:"updated_by_id"`
}
type Input struct {
	UserID  string `json:"user_id"`
	Version int32  `json:"version"`
	Active  bool   `json:"is_active"`
	Reason  string `json:"reason"`
}

func (i Input) Validate() (Input, error) {
	var e error
	i.UserID, e = ID(i.UserID)
	if e != nil {
		return i, e
	}
	i.Reason = strings.TrimSpace(i.Reason)
	if i.Version < 0 {
		return i, Invalid("version", "Use the current designation version")
	}
	if i.Reason == "" || utf8.RuneCountInString(i.Reason) > 2000 {
		return i, Invalid("reason", "Record a change reason of at most 2000 characters")
	}
	return i, nil
}
func Change(previous *Designation, input Input, id, actor string, now time.Time) (Designation, string, error) {
	version := int32(0)
	if previous != nil {
		version = previous.Version
	}
	if input.Version != version {
		return Designation{}, "", Stale
	}
	if previous == nil && !input.Active {
		return Designation{}, "", Invalid("is_active", "Designate an active Director before revoking")
	}
	v := Designation{ID: id, CreatedAt: now, CreatedBy: &actor}
	action := "designate"
	if previous != nil {
		v = *previous
		if previous.UserID == input.UserID && previous.Active == input.Active {
			return v, "", Invalid("user_id", "No designation change to save")
		}
		action = "change"
		if !input.Active {
			action = "revoke"
		} else if !previous.Active && previous.UserID == input.UserID {
			action = "reactivate"
		}
	}
	v.UserID = input.UserID
	v.Active = input.Active
	v.Version = version + 1
	v.UpdatedAt = now
	v.UpdatedBy = &actor
	return v, action, nil
}

type Event struct {
	ID      string       `json:"id"`
	Action  string       `json:"action"`
	Reason  string       `json:"reason"`
	Before  *Designation `json:"before"`
	After   Designation  `json:"after"`
	ActorID string       `json:"created_by_id"`
	At      time.Time    `json:"created_at"`
}
type Workspace struct {
	Record    *Designation `json:"record"`
	User      *User        `json:"user"`
	Effective bool         `json:"is_effective"`
	CanWrite  bool         `json:"can_write"`
	History   []Event      `json:"history"`
}
