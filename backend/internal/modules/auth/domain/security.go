package domain

import (
	"time"
)

type Session struct {
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time
	Current   bool
}

type EmailChange struct {
	Hash, UserID, Email, PreviousEmail, ActorID string
	CredentialVersion                           int64
	ExpiresAt                                   time.Time
	Consumed                                    bool
}

type AdministrationChange struct {
	ID         int64
	EntityType string
	EntityID   string
	Action     string
	ActorID    string
	Before     []byte
	After      []byte
	OccurredAt time.Time
}
