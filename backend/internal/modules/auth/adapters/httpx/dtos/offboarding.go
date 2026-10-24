package dtos

import (
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"io"
	"strings"
	"time"
)

type HandoverResponse struct {
	Work            WorkResponse `json:"work"`
	ReplacementID   string       `json:"replacement_id"`
	ApprovedVersion int64        `json:"approved_version"`
}
type OffboardingResponse struct {
	ID           string             `json:"id"`
	UserID       string             `json:"user_id"`
	ManagerID    string             `json:"manager_id"`
	State        string             `json:"state"`
	Reason       string             `json:"reason"`
	Version      int64              `json:"version"`
	Items        []HandoverResponse `json:"items"`
	ApprovalByID string             `json:"approval_by_id"`
	ApprovedAt   *time.Time         `json:"approved_at"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

func Offboarding(c domain.OffboardingCase) OffboardingResponse {
	items := []HandoverResponse{}
	for _, v := range c.Items {
		items = append(items, HandoverResponse{Work(v.Responsibility), v.ReplacementID, v.ApprovedVersion})
	}
	return OffboardingResponse{c.ID, c.UserID, c.ManagerID, c.State, c.Reason, c.Version, items, c.ApprovalByID, c.ApprovedAt, c.CreatedAt, c.UpdatedAt}
}

type DepartureRequest struct{ UserID, Reason string }

func ParseDeparture(r io.Reader) (DepartureRequest, error) {
	var p struct {
		UserID string `json:"user_id"`
		Reason string `json:"reason"`
	}
	_, e := Decode(r, &p, "user_id", "reason")
	if e != nil {
		return DepartureRequest{}, e
	}
	if !organisationID.MatchString(p.UserID) {
		return DepartureRequest{}, Invalid("user_id", "Valid employee identifier required", "value_error")
	}
	if !domain.ValidResetReason(strings.TrimSpace(p.Reason)) {
		return DepartureRequest{}, Invalid("reason", "Use 10 to 1000 characters", "value_error")
	}
	return DepartureRequest{strings.ToLower(p.UserID), strings.TrimSpace(p.Reason)}, nil
}
func ParseHandover(r io.Reader) (domain.HandoverPlan, error) {
	var p struct {
		Version    *int64 `json:"version"`
		Reason     string `json:"reason"`
		Selections []struct {
			Source        string `json:"source"`
			Kind          string `json:"kind"`
			ID            string `json:"id"`
			ReplacementID string `json:"replacement_id"`
		} `json:"selections"`
	}
	_, e := Decode(r, &p, "version", "reason", "selections")
	if e != nil {
		return domain.HandoverPlan{}, e
	}
	if p.Version == nil || *p.Version < 1 {
		return domain.HandoverPlan{}, Invalid("version", "Current case version is required", "value_error")
	}
	if !domain.ValidResetReason(strings.TrimSpace(p.Reason)) {
		return domain.HandoverPlan{}, Invalid("reason", "Use 10 to 1000 characters", "value_error")
	}
	if len(p.Selections) > 200 {
		return domain.HandoverPlan{}, Invalid("selections", "At most 200 updates per request", "value_error")
	}
	in := domain.HandoverPlan{Version: *p.Version, Reason: strings.TrimSpace(p.Reason), Selections: []domain.HandoverSelection{}}
	for _, v := range p.Selections {
		if len(v.Source) > 40 || len(v.Kind) > 80 || len(v.ID) > 100 || (v.ReplacementID != "" && !organisationID.MatchString(v.ReplacementID)) {
			return in, Invalid("selections", "Invalid responsibility or replacement reference", "value_error")
		}
		in.Selections = append(in.Selections, domain.HandoverSelection{Source: v.Source, Kind: v.Kind, ID: strings.ToLower(v.ID), ReplacementID: strings.ToLower(v.ReplacementID)})
	}
	return in, nil
}

type ReplacementResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	Superuser   bool     `json:"is_superuser"`
}

func Replacement(u domain.User) ReplacementResponse {
	return ReplacementResponse{u.ID, strings.TrimSpace(u.FirstName+" "+u.LastName) + " (" + u.Username + ")", domain.EffectivePermissions(u), u.IsSuperuser}
}

type OffboardingChangeResponse struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	ActorID   string    `json:"actor_id"`
	ActorName string    `json:"actor_name"`
	Version   int64     `json:"version"`
	At        time.Time `json:"created_at"`
}

func OffboardingChange(v domain.OffboardingChange) OffboardingChangeResponse {
	return OffboardingChangeResponse{v.ID, v.Action, v.Reason, v.ActorID, v.ActorName, v.Version, v.At}
}
