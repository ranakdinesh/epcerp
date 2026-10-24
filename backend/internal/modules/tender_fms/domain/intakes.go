package domain

import (
	"regexp"
	"strings"
	"time"
)

type Error struct {
	Status int
	Detail string
}

func (e *Error) Error() string { return e.Detail }

var Unauthorized = &Error{401, "Invalid or expired credentials"}
var Forbidden = &Error{403, "Permission denied"}
var Missing = &Error{404, "Tender intake not found"}

func Invalid(s string) error { return &Error{422, s} }

type Principal struct {
	ID                string
	Active, Superuser bool
	Permissions       []string
}

func (p Principal) Allows(code string) bool {
	if !p.Active || p.ID == "" {
		return false
	}
	if p.Superuser {
		return true
	}
	for _, v := range p.Permissions {
		if v == code {
			return true
		}
	}
	return false
}
func (p Principal) Can(owner, action string) bool {
	if !p.Active || p.ID == "" {
		return false
	}
	return owner == p.ID || p.Allows("tenders.portfolio."+action)
}

type Intake struct {
	RecordKind string    `json:"-"`
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	OwnerID    string    `json:"owner_id"`
	Version    int32     `json:"version"`
	Active     bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	CreatedBy  *string   `json:"created_by_id"`
	UpdatedBy  *string   `json:"updated_by_id"`
	Documents  []Source  `json:"documents"`
}
type Source struct {
	ID           string   `json:"id"`
	Filename     string   `json:"filename"`
	Format       string   `json:"format"`
	Status       string   `json:"status"`
	Size         int64    `json:"size_bytes"`
	SHA256       string   `json:"sha256"`
	Version      int32    `json:"version"`
	Attempts     int32    `json:"attempts"`
	Error        string   `json:"error"`
	SegmentCount int32    `json:"segment_count"`
	Warnings     []string `json:"warnings"`
}
type Artifact struct {
	Data              []byte
	Name, ContentType string
}
type Event struct {
	Reason     string    `json:"reason,omitempty"`
	ID         string    `json:"id"`
	Action     string    `json:"action"`
	ActorID    string    `json:"actor_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	DocumentID string    `json:"document_id"`
	Version    int32     `json:"version,omitempty"`
}

func ID(v string) (string, error) {
	if !regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`).MatchString(v) {
		return "", Invalid("Invalid tender/document identity")
	}
	return strings.ToLower(v), nil
}
