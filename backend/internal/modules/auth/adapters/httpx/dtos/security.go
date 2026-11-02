package dtos

import (
	"encoding/json"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"time"
)

type SessionResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Current   bool      `json:"current"`
}

func Sessions(in []domain.Session) []SessionResponse {
	out := []SessionResponse{}
	for _, v := range in {
		out = append(out, SessionResponse{v.ID, v.CreatedAt, v.ExpiresAt, v.Current})
	}
	return out
}

type AdministrationChangeResponse struct {
	ID         int64           `json:"id"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	Action     string          `json:"action"`
	ActorID    string          `json:"actor_id"`
	Before     json.RawMessage `json:"before"`
	After      json.RawMessage `json:"after"`
	OccurredAt time.Time       `json:"occurred_at"`
}

func AdministrationChanges(in []domain.AdministrationChange) []AdministrationChangeResponse {
	out := []AdministrationChangeResponse{}
	for _, v := range in {
		out = append(out, AdministrationChangeResponse{v.ID, v.EntityType, v.EntityID, v.Action, v.ActorID, v.Before, v.After, v.OccurredAt})
	}
	return out
}
