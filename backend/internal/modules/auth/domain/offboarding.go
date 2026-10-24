package domain

import "time"

type HandoverItem struct {
	Responsibility  Responsibility
	ReplacementID   string
	ApprovedVersion int64
}
type OffboardingCase struct {
	ID, UserID, ManagerID, State, Reason, ApprovalByID string
	ProfileVersion, Version                            int64
	Items                                              []HandoverItem
	ApprovedAt                                         *time.Time
	CreatedAt, UpdatedAt                               time.Time
	CreatedByID, UpdatedByID                           string
}
type HandoverSelection struct{ Source, Kind, ID, ReplacementID string }
type HandoverPlan struct {
	Version    int64
	Reason     string
	Selections []HandoverSelection
}

var ErrOffboardingRequired = &Error{Conflict, "Start an offboarding case. The reporting manager must approve completed handovers before an administrator can deactivate this user."}
var ErrOffboardingStale = &Error{Conflict, "Handover or reporting information changed. Refresh the case and obtain manager approval again."}

func EligibleReplacement(u User, departing string, w Responsibility) bool {
	if u.ID == departing || !u.IsActive || u.MustChangePassword {
		return false
	}
	for _, p := range w.RequiredPermissions {
		if !HasPermission(u, p) {
			return false
		}
	}
	return true
}
func (c OffboardingCase) Open() bool { return c.State == "planning" || c.State == "approved" }

type OffboardingChange struct {
	ID, Action, Reason, ActorID, ActorName string
	Version                                int64
	At                                     time.Time
}
