package domain

type WorkflowTaskQuery struct {
	Page, Size          int32
	Search, Role, State string
}
type WorkflowAssignedTask struct {
	ID            string   `json:"id"`
	ProjectID     string   `json:"project_id"`
	ProjectName   string   `json:"project_name"`
	PackageID     string   `json:"package_id"`
	Code          string   `json:"package_code"`
	Number        int      `json:"step_number"`
	Title         string   `json:"title"`
	Phase         string   `json:"phase"`
	Relation      string   `json:"relation"`
	State         string   `json:"coordination_state"`
	PlannedDate   string   `json:"planned_date"`
	ResolutionDue string   `json:"resolution_due"`
	BlockReason   string   `json:"block_reason"`
	Available     bool     `json:"available"`
	Blockers      []string `json:"blockers"`
	Version       int32    `json:"package_version"`
}
type WorkflowTaskPage struct {
	Rows  []WorkflowAssignedTask
	Total int64
}

// Progress packages remain workflow-owned. Inspect keeps blockers visible while
// the consuming report independently decides which current evidence earns weight.
type WorkflowProgressPackage struct {
	Package WorkflowPackage
	Source  WorkflowSource
}
type WorkflowProgressPage struct {
	Rows  []WorkflowProgressPackage
	Total int64
}
