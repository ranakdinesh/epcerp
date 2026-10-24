package domain

import (
	"github.com/ranakdinesh/epcERP/internal/identity"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Record is V0 metadata only. It carries no source bytes, commercial terms or financial rights.
type Record struct {
	ID, Name, SalesCreatorID, ManagerID, ManagerAssignmentID string
	BusinessVersion, ContentRevision, AssignmentVersion      int64
	Active                                                   bool
	CreatedAt, UpdatedAt                                     time.Time
	CreatedBy, UpdatedBy                                     *string
}
type NamedInput struct{ Name, ManagerID string }

func (in NamedInput) Normalize() (NamedInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	for _, r := range in.Name {
		if unicode.IsControl(r) {
			return in, Invalid("Tender name cannot contain control characters")
		}
	}
	if !utf8.ValidString(in.Name) || utf8.RuneCountInString(in.Name) < 2 || utf8.RuneCountInString(in.Name) > 255 {
		return in, Invalid("Tender name must contain 2 to 255 characters")
	}
	id, e := ID(in.ManagerID)
	in.ManagerID = id
	return in, e
}

type Candidate struct{ ID, DisplayName string }
type EligibleIdentity struct {
	Principal         identity.Principal
	Ready, Designated bool
	DisplayName       string
}

// Manager permissions are the approved full profile. Pending evaluation permissions
// are deliberately not registered until their backed slice exists: fail closed.
func ManagerPermissions() []string {
	return []string{
		"tenders.register.view", "tenders.register.read", "tenders.register.write",
		"tenders.processing.read", "tenders.processing.write", "tenders.register.assign",
		"tenders.evaluation.read", "tenders.evaluation.write",
		"communication.inbox.read", "communication.inbox.write",
	}
}
func (u EligibleIdentity) CanManage() bool {
	if !u.Ready || !u.Designated {
		return false
	}
	for _, code := range ManagerPermissions() {
		if !u.Principal.Allows(code) {
			return false
		}
	}
	return true
}
func CanCapture(p identity.Principal) bool {
	for _, code := range []string{"tenders.register.view", "tenders.register.read", "tenders.register.write", "tenders.processing.write", "tenders.register.assign"} {
		if !p.Allows(code) {
			return false
		}
	}
	return true
}
func (v Record) CanRead(p identity.Principal) bool {
	return p.Allows("tenders.register.view") && p.Allows("tenders.register.read") &&
		(v.SalesCreatorID == p.ID || v.ManagerID == p.ID || p.Allows("tenders.portfolio.read"))
}

// ParticipantGrant confers V0 visibility only. It never grants source/price/Finance rights.
type ParticipantGrant struct {
	UserID, Reason  string
	ExpectedVersion int64 // Zero is valid only for the first grant.
	Active          bool
	ValidUntil      *time.Time
}

func (in ParticipantGrant) Normalize() (ParticipantGrant, error) {
	id, e := ID(in.UserID)
	if e != nil {
		return in, e
	}
	in.UserID = id
	in.Reason = strings.TrimSpace(in.Reason)
	if in.ExpectedVersion < 0 || in.ExpectedVersion >= 9007199254740991 || utf8.RuneCountInString(in.Reason) < 10 || utf8.RuneCountInString(in.Reason) > 1000 {
		return in, Invalid("Current grant version and a reason of 10 to 1000 characters are required")
	}
	return in, nil
}
