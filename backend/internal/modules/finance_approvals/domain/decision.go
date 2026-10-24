package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type Timestamp = time.Time
type Error struct {
	Status         int
	Field, Message string
}

func (e *Error) Error() string  { return e.Message }
func Invalid(f, m string) error { return &Error{422, f, m} }

var Unauthorized = &Error{401, "", "Invalid or expired credentials"}
var Forbidden = &Error{403, "", "Finance approval permission or project scope denied"}
var Conflict = &Error{409, "", "This budget revision already has a Finance decision; reload"}

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

type BudgetRequest struct {
	ProjectID                             string
	Version, Revision                     int32
	Amount, Currency, Fingerprint, Reason string
}
type Decision struct {
	ID, ProjectID, ActorID                string
	SourceVersion, SourceRevision         int32
	Amount, Currency, Fingerprint, Reason string
	CreatedAt                             Timestamp
}

var identifier = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var amount = regexp.MustCompile(`^[0-9]{1,14}(?:\.[0-9]{1,2})?$`)
var currency = regexp.MustCompile(`^[A-Z]{3}$`)
var fingerprint = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (q BudgetRequest) Validate() (BudgetRequest, error) {
	if !identifier.MatchString(q.ProjectID) || q.Version < 1 || q.Revision < 1 {
		return q, Invalid("source", "Approve a saved current budget revision")
	}
	if !amount.MatchString(q.Amount) || strings.Trim(q.Amount, "0.") == "" {
		return q, Invalid("amount", "Approve a positive exact budget amount")
	}
	if !currency.MatchString(q.Currency) || !fingerprint.MatchString(q.Fingerprint) {
		return q, Invalid("source", "Budget currency or evidence fingerprint is invalid")
	}
	q.Reason = strings.TrimSpace(q.Reason)
	if q.Reason == "" || utf8.RuneCountInString(q.Reason) > 2000 {
		return q, Invalid("reason", "Record an approval reason of at most 2000 characters")
	}
	return q, nil
}
