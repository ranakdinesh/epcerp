package domain

import "strings"

type OwnershipInput struct {
	OwnerID, Reason string
	Version         int32
}

func (v OwnershipInput) Validate() error {
	if _, e := ID(v.OwnerID); e != nil {
		return e
	}
	if v.Version < 1 || len([]rune(strings.TrimSpace(v.Reason))) < 10 || len([]rune(v.Reason)) > 1000 {
		return Invalid("Current version and a reason of 10 to 1000 characters are required")
	}
	return nil
}

type OwnershipCandidate struct{ ID, Name string }

func (v OwnershipInput) Normalized() OwnershipInput {
	v.OwnerID = strings.ToLower(v.OwnerID)
	v.Reason = strings.TrimSpace(v.Reason)
	return v
}
