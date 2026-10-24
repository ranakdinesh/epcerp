// Package finance_approvals exposes the finance approval capability boundary.
package finance_approvals

// Module identifies the approval capability; its published adapters are injected
// into the owning project and purchasing transactions by their module composers.
type Module struct{}

func New() *Module { return &Module{} }
