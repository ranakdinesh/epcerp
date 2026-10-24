package domain

// OrdersClientCleared binds external client sign-off to the exact prepared draft.
func OrdersClientCleared(o OrdersOrder) bool {
	a := o.ClientApproval
	return o.DraftRevision > 0 && a.Status == "approved" && a.DraftRevision == o.DraftRevision && a.Submission != nil && a.Decision != nil && a.Decision.Status == "approved" && a.Decision.FileID != "" && a.RecordedBy != "" && a.RecordedAt != nil
}
