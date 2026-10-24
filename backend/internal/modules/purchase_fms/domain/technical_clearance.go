package domain

// Clearance is an internal technical fact, never an actor authorization grant.
// It is recalculated against current owned evidence in the consumer transaction.
type TechnicalClearance struct {
	TransmittalReference string              `json:"transmittal_reference"`
	CaseID               string              `json:"case_id"`
	ProjectID            string              `json:"project_id"`
	PackageID            string              `json:"package_id"`
	Version              int32               `json:"version"`
	Revision             int32               `json:"revision"`
	Source               TechnicalSource     `json:"source"`
	Approval             TechnicalApproval   `json:"approval"`
	Documents            []TechnicalDocument `json:"documents"`
}

func TechnicalClearanceFact(c TechnicalCase, current TechnicalSource, files []TechnicalFile) (*TechnicalClearance, error) {
	if !TechnicalCurrentReady(c, current, files) {
		return nil, TechnicalInvalid("technical_approval", "Current client-approved full kit, signed QAP and closed conditions are required before PO release")
	}
	return &TechnicalClearance{TransmittalReference: c.Submission.Reference, CaseID: c.ID, ProjectID: c.ProjectID, PackageID: c.PackageID, Version: c.Version, Revision: c.Revision, Source: TechnicalCopy(c).Source, Approval: *c.Approval, Documents: append([]TechnicalDocument{}, c.Documents...)}, nil
}
