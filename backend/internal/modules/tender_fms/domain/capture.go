package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

type CaptureInput struct {
	Name, ReceiptID, ManagerID   string
	ReceiptVersion               int64
	ExternalReference, SourceURL *string
}
type ChangeManager struct {
	Version, AssignmentVersion int64
	ManagerID, Reason          string
}
type Patch struct {
	SetReference, SetURL         bool
	Version                      int64
	Name                         string
	ExternalReference, SourceURL *string
}
type CaptureOperation struct{ Actor, Command, Context, Key, Fingerprint, TenderID string }
type PackageRef struct{ ID, ClaimID, OwnerID, TenderID string }
type Delivery struct{ ActionID, NoticeID, EmailJobID, EmailState string }
type Captured struct {
	ManagerName                          string
	CanEdit, CanAssign                   bool
	Record                               Record
	ProvisionID, ProvisionState, Blocker string
	ProvisionVersion                     int64
	ProjectID, EstimateID                string
	Deliveries                           []Delivery
}
type AssignmentAction struct {
	ID, TenderID, AssignmentID, RecipientID, EventID, ActorID string
	AssignmentVersion                                         int64
	At                                                        time.Time
}

func CanonicalID(s string) error {
	id, e := ID(s)
	if e != nil {
		return e
	}
	if id != s {
		return Invalid("Use canonical lowercase UUIDs")
	}
	return nil
}
func (in CaptureInput) Normalize() (CaptureInput, error) {
	for _, id := range []string{in.ReceiptID, in.ManagerID} {
		if e := CanonicalID(id); e != nil {
			return in, e
		}
	}
	name, e := (NamedInput{Name: in.Name, ManagerID: in.ManagerID}).Normalize()
	if e != nil {
		return in, e
	}
	in.Name = name.Name
	if in.ReceiptVersion < 1 || in.ReceiptVersion > 9007199254740991 {
		return in, Invalid("Current receipt version is required")
	}
	if in.ExternalReference != nil {
		s := *in.ExternalReference
		if !utf8.ValidString(s) || utf8.RuneCountInString(s) < 1 || utf8.RuneCountInString(s) > 255 || strings.ContainsAny(s, "\x00\r\n") {
			return in, Invalid("External reference must contain 1 to 255 characters")
		}
	}
	if in.SourceURL != nil && (len(*in.SourceURL) > 2048 || strings.ContainsAny(*in.SourceURL, "\x00\r\n")) {
		return in, Invalid("Invalid source URL")
	}
	return in, nil
}
func (in ChangeManager) Normalize() (ChangeManager, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if e := CanonicalID(in.ManagerID); e != nil {
		return in, e
	}
	if in.Version < 1 || in.AssignmentVersion < 1 || in.Version >= 9007199254740991 || in.AssignmentVersion >= 9007199254740991 || utf8.RuneCountInString(in.Reason) < 10 || utf8.RuneCountInString(in.Reason) > 1000 {
		return in, Invalid("Current versions and a reason of 10 to 1000 characters are required")
	}
	return in, nil
}

type CaptureError struct {
	Status       int
	Code, Detail string
}

func (e *CaptureError) Error() string { return e.Detail }
func CaptureFailure(status int, code, detail string) error {
	return &CaptureError{status, code, detail}
}

func ValidateRegisterQuery(search, sort string) error {
	if !utf8.ValidString(search) || utf8.RuneCountInString(search) > 200 || strings.ContainsAny(search, "\x00\r\n") || (sort != "name" && sort != "created") {
		return Invalid("Search or sort is outside supported bounds")
	}
	return nil
}
