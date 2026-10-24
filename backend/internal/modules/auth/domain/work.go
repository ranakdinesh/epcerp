package domain

// Responsibility contains operational status only: no amounts, documents or credentials.
type Responsibility struct {
	Source, Kind, ID, OwnerID, Title, State, Link, DueDate string
	ProjectID                                              string
	Version                                                int64
	RequiredPermissions                                    []string
}

func (w Responsibility) Same(other Responsibility) bool {
	return w.Source == other.Source && w.Kind == other.Kind && w.ID == other.ID
}
