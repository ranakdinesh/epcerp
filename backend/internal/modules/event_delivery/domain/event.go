package domain

import (
	"errors"
	"regexp"
	"time"
)

var (
	ErrInvalid             = errors.New("invalid event or delivery metadata")
	ErrConflict            = errors.New("event replay conflicts with recorded metadata")
	ErrLeaseLost           = errors.New("delivery lease expired or was replaced")
	ErrTransactionRequired = errors.New("event publication requires the source transaction")
)
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var code = regexp.MustCompile(`^[a-z][a-z0-9_.]{0,63}$`)

// Event holds references only. Sensitive detail remains in its authorized source.
type Event struct {
	ID, Module, EntityType, EntityID, Action       string
	EntityVersion                                  int64
	ActorID, ActorKind, CorrelationID, CausationID string
	ScopeKind, ScopeID, Visibility                 string
	OccurredAt                                     time.Time
}
type Target struct {
	Consumer    string
	MaxAttempts int32
}
type Claim struct {
	ID, Consumer, LeaseToken string
	Attempt                  int32
	Event                    Event
}
type Policy struct{ LeaseSeconds, HandlerSeconds, RetrySeconds, MaxRetrySeconds int32 }

func ValidID(id string) bool      { return uuid.MatchString(id) }
func ValidCode(value string) bool { return code.MatchString(value) }
func (e Event) Normalize() (Event, error) {
	if !ValidID(e.ID) || !ValidID(e.EntityID) || !ValidID(e.CorrelationID) ||
		(e.ActorID != "" && !ValidID(e.ActorID)) || (e.CausationID != "" && !ValidID(e.CausationID)) ||
		!ValidCode(e.Module) || !ValidCode(e.EntityType) || !ValidCode(e.Action) || e.EntityVersion < 1 ||
		e.OccurredAt.IsZero() || e.OccurredAt.Year() < 2000 || e.OccurredAt.Year() > 9999 || e.Visibility != "restricted" {
		return Event{}, ErrInvalid
	}
	switch e.ActorKind {
	case "user":
		if e.ActorID == "" {
			return Event{}, ErrInvalid
		}
	case "system":
		if e.ActorID != "" {
			return Event{}, ErrInvalid
		}
	default:
		return Event{}, ErrInvalid
	}
	switch e.ScopeKind {
	case "platform":
		if e.ScopeID != "" {
			return Event{}, ErrInvalid
		}
	case "project", "tender":
		if !ValidID(e.ScopeID) {
			return Event{}, ErrInvalid
		}
	default:
		return Event{}, ErrInvalid
	}
	e.OccurredAt = e.OccurredAt.UTC().Truncate(time.Microsecond)
	return e, nil
}
func ValidateTargets(targets []Target) error {
	if len(targets) > 32 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, t := range targets {
		if !ValidCode(t.Consumer) || t.MaxAttempts < 1 || t.MaxAttempts > 20 || seen[t.Consumer] {
			return ErrInvalid
		}
		seen[t.Consumer] = true
	}
	return nil
}
func (p Policy) Validate() error {
	if p.LeaseSeconds < 2 || p.LeaseSeconds > 3600 || p.HandlerSeconds < 1 || p.HandlerSeconds >= p.LeaseSeconds ||
		p.RetrySeconds < 1 || p.MaxRetrySeconds < p.RetrySeconds || p.MaxRetrySeconds > 86400 {
		return ErrInvalid
	}
	return nil
}
func (p Policy) RetryDelay(attempt int32) int32 {
	delay := int64(p.RetrySeconds)
	for i := int32(1); i < attempt && delay < int64(p.MaxRetrySeconds); i++ {
		delay *= 2
	}
	if delay > int64(p.MaxRetrySeconds) {
		delay = int64(p.MaxRetrySeconds)
	}
	return int32(delay)
}
func (p Policy) HandlerTimeout() time.Duration { return time.Duration(p.HandlerSeconds) * time.Second }
func Equal(a, b Event) bool {
	return a.ID == b.ID && a.Module == b.Module && a.EntityType == b.EntityType && a.EntityID == b.EntityID && a.Action == b.Action && a.EntityVersion == b.EntityVersion && a.ActorID == b.ActorID && a.ActorKind == b.ActorKind && a.CorrelationID == b.CorrelationID && a.CausationID == b.CausationID && a.ScopeKind == b.ScopeKind && a.ScopeID == b.ScopeID && a.Visibility == b.Visibility && a.OccurredAt.Equal(b.OccurredAt)
}
