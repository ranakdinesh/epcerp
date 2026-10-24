package domain

// Status is the public operational state, without infrastructure details.
type Status string

const (
	OK          Status = "ok"
	Unavailable Status = "unavailable"
)
