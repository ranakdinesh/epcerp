package domain

// ApprovedSource is the effective financial baseline, never a draft projection.
// Preparing a new revision retains this approved source until its replacement is approved.
type ApprovedSource struct {
	ProjectID   string
	Revision    int32
	Currency    string
	DecisionID  string
	Allocations []ApprovedAllocation
}
type ApprovedAllocation struct{ BOID, LineID, Amount string }

func EffectiveApprovedSource(record *Budget) *ApprovedSource {
	if record == nil || !record.Active || record.Approved == nil {
		return nil
	}
	approved := record.Approved
	source := &ApprovedSource{ProjectID: record.ProjectID, Revision: approved.Revision, Currency: approved.Data.Currency, DecisionID: approved.DecisionID, Allocations: []ApprovedAllocation{}}
	for _, a := range approved.Data.Allocations {
		if a.Active {
			source.Allocations = append(source.Allocations, ApprovedAllocation{BOID: a.BOID, LineID: a.LineID, Amount: a.Amount})
		}
	}
	return source
}
