package domain

import (
	"time"
)

type Correction struct {
	Name, ClientID, Reason string
	ExpectedVersion        int32
	Capture
}

func (c Correction) Validate() (Correction, error) {
	var e error
	c.Name, e = Text("name", c.Name, 255)
	if e != nil {
		return c, e
	}
	if len([]rune(c.Name)) < 3 {
		return c, &Invalid{"name", "Enter at least three characters"}
	}
	c.ClientID, e = ID(c.ClientID)
	if e != nil {
		return c, &Invalid{"client_id", "Select an active client"}
	}
	c.Reason, e = Text("reason", c.Reason, 2000)
	if e != nil {
		return c, e
	}
	if c.ExpectedVersion < 1 {
		return c, &Invalid{"expected_version", "Current project version is required"}
	}
	c.Capture, e = c.Capture.ValidateDraft()
	return c, e
}

type MetadataChange struct {
	ID, ProjectID, Reason, ActorID string
	Version                        int32
	Before, After                  Project
	OccurredAt                     time.Time
}

func CanCorrect(actor Principal, p Project) bool {
	return actor.Allows("projects.master.read") && actor.Allows("projects.master.write") && (actor.ID == p.ManagerID || actor.Allows("projects.portfolio.write"))
}
