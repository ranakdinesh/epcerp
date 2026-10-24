package domain

func TechnicalChecklist(ids []string) ([]TechnicalDocument, error) {
	if len(ids) != 6 {
		return nil, TechnicalInvalid("documents", "Provide six owned checklist identities")
	}
	rows := []TechnicalDocument{
		{Kind: "ga-drawing", Title: "General Arrangement (GA) Drawing", Category: "Engineering Drawings", Standard: "Tender technical data sheet and applicable standards", Required: true},
		{Kind: "tds", Title: "Technical Data Sheet (TDS)", Category: "Specifications", Standard: "Tender technical parameter schedule", Required: true},
		{Kind: "structural-calc", Title: "Structural and Foundation Design Calculation", Category: "Design Calculation", Standard: "Applicable structural and foundation design requirements", Required: true},
		{Kind: "type-test", Title: "Type Test Certificates", Category: "Quality / Lab Reports", Standard: "Applicable accredited laboratory and tender test requirements", Required: true},
		{Kind: "qap", Title: "Quality Assurance Plan (QAP)", Category: "Inspection Hold Points", Standard: "Factory inspection and customer witness points", Required: true},
		{Kind: "mtc-raw", Title: "Raw Material Test Certificates (MTC)", Category: "Material Traceability", Standard: "Applicable material certificates", Required: false},
	}
	seen := map[string]bool{}
	for i, id := range ids {
		if e := TechnicalID(id); e != nil {
			return nil, e
		}
		if seen[id] {
			return nil, TechnicalInvalid("documents", "Checklist identities must be unique")
		}
		seen[id] = true
		rows[i].ID = id
		rows[i].Status = "pending"
		rows[i].Revision = "Rev 0"
		rows[i].Active = true
	}
	return rows, nil
}
