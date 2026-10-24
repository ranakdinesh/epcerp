package domain

// Released is immutable commercial authority for downstream capabilities, not a
// grant of actor access. Consumers enforce current project and task permissions.
type SourcingReleased struct {
	CaseID             string            `json:"case_id"`
	ProjectID          string            `json:"project_id"`
	PackageID          string            `json:"package_id"`
	Version            int32             `json:"version"`
	Source             SourcingSource    `json:"source"`
	Terms              string            `json:"loi_terms"`
	LOIReference       string            `json:"loi_reference"`
	Offer              SourcingOffer     `json:"offer"`
	Total              string            `json:"total"`
	FinancialReference string            `json:"financial_reference"`
	ReleasedAt         SourcingTimestamp `json:"released_at"`
}

func SourcingReleasedFact(c SourcingCase) (*SourcingReleased, error) {
	if !c.Active || c.State != "released" {
		return nil, nil
	}
	if c.Decision == nil || c.ReleasedAt == nil || c.LOIReference == "" || c.Decision.FinancialReference == "" || c.ApprovedBy == "" || c.ApprovedAt == nil || c.ApprovedBy == c.ProposedBy || c.ApprovedBy != c.ApproverID || c.Decision.Offer.SupplierID == "" {
		return nil, SourcingInvalid("loi", "Released commercial evidence is incomplete")
	}
	c = SourcingCopy(c)
	total, e := SourcingTotal(c.Decision.Offer, c.Decision.Source.Quantity)
	if e != nil {
		return nil, e
	}
	return &SourcingReleased{CaseID: c.ID, ProjectID: c.ProjectID, PackageID: c.PackageID, Version: c.Version, Source: c.Decision.Source, Terms: c.Decision.Terms, LOIReference: c.LOIReference, Offer: c.Decision.Offer, Total: total, FinancialReference: c.Decision.FinancialReference, ReleasedAt: *c.ReleasedAt}, nil
}

// CurrentReleasedFact revalidates the original commercial authority without
// changing historical LOI evidence or granting a consumer actor any permission.
func SourcingCurrentReleasedFact(c SourcingCase, current SourcingSource, supplier *SourcingSupplier, proof *SourcingProof, today string) (*SourcingReleased, error) {
	fact, err := SourcingReleasedFact(c)
	if err != nil || fact == nil {
		return fact, err
	}
	if err = current.Guard(); err != nil {
		return nil, err
	}
	if !sourcingSameSource(fact.Source, current) {
		return nil, SourcingInvalid("source", "Released LOI no longer matches the current item demand")
	}
	if err = SourcingSupplierGuard(supplier, true); err != nil {
		return nil, err
	}
	offer, err := SourcingSelected(c, today)
	if err != nil {
		return nil, err
	}
	if *offer != fact.Offer {
		return nil, SourcingInvalid("loi", "Released selection no longer matches its approved snapshot")
	}
	if err = sourcingProofGuard(c, fact.Offer, proof); err != nil {
		return nil, err
	}
	if fact.FinancialReference != proof.Reference+":"+proof.Fingerprint {
		return nil, SourcingInvalid("financial_approval", "Financial approval changed after LOI release")
	}
	return fact, nil
}
