package domain

// Error carries a purchase rule failure. Workflow-specific sentinels keep their
// messages and identity; adapters translate the retained API error classification.
type Error struct {
	Status         int
	Field, Message string
}

func (e *Error) Error() string { return e.Message }
