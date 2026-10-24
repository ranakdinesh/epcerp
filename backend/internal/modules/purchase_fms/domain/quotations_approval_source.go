package domain

// ApprovalSource is a purchasing evidence projection, never an approval. Groups
// include those with no quotation so consumers cannot infer readiness from a
// nonempty list of priced vendors. Prices remain the saved revision calculations.
type QuotationsApprovalSource struct {
	ProjectID string
	BOID      string
	Currency  string
	Available bool
	Groups    []QuotationsApprovalGroup
	Quotes    []QuotationsComparison
}

type QuotationsApprovalGroup struct {
	ID            string
	Name          string
	ScopeRevision int32
}
