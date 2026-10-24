package domain

// ReleasedFact returns owned historical evidence, not current downstream authority.
// Consumers authorize their own use case and check their own live prerequisites.
func OrdersReleasedFact(record *OrdersOrder, project, pkg string) (*OrdersOrder, error) {
	if record == nil {
		return nil, OrdersMissing
	}
	if !record.Active || record.ProjectID != project || record.PackageID != pkg || record.Source.ProjectID != project || record.Source.PackageID != pkg {
		return nil, OrdersMissing
	}
	if record.State != "released" || record.Number == "" || record.Version < 2 || record.ReleasedAt == nil || record.ReleasedAt.IsZero() || record.ReleasedBy == "" || !OrdersDate(record.Details.Date) {
		return nil, OrdersInvalid("purchase_order", "Release the actual purchase order before recording vendor acceptance")
	}
	for _, id := range []string{record.ID, record.ProjectID, record.PackageID, record.ReleasedBy} {
		if err := OrdersID(id); err != nil {
			return nil, err
		}
	}
	if err := record.Source.Guard(); err != nil {
		return nil, err
	}
	out := OrdersCopy(*record)
	return &out, nil
}
