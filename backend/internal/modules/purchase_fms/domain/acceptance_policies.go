package domain

func AcceptanceAccessGuard(a Principal, c AcceptanceAccess, write bool) error {
	if !a.Active || a.ID == "" {
		return AcceptanceUnauthorized
	}
	if !c.Read || write && (!c.Write || !c.Work) {
		return AcceptanceForbidden
	}
	return nil
}
func (s AcceptanceSource) Guard() error {
	if !s.Available || len(s.Blockers) > 0 || s.Order == nil {
		return AcceptanceInvalid("source", "An actual released PO and current project/task prerequisites are required")
	}
	o := s.Order
	if o.ProjectID != s.ProjectID || o.PackageID != s.PackageID {
		return AcceptanceInvalid("source", "Released PO belongs to another project or package")
	}
	for _, v := range []string{o.ID, o.ProjectID, o.PackageID, o.SupplierID, o.ReleasedBy} {
		if e := AcceptanceID(v); e != nil {
			return e
		}
	}
	if o.Version < 2 || o.Number == "" || !AcceptanceDate(o.Date) || o.ReleasedAt.IsZero() || o.SupplierName == "" || o.Quantity == "" || o.Total == "" || o.UOM == "" || o.Currency == "" {
		return AcceptanceInvalid("purchase_order", "The released PO evidence is incomplete")
	}
	return nil
}
func AcceptanceValidate(q AcceptanceCommand) (AcceptanceCommand, error) {
	if q.Version < 0 {
		return q, AcceptanceInvalid("version", "Use the current nonnegative version")
	}
	if q.Action != "record" {
		return q, AcceptanceInvalid("action", "Choose record")
	}
	var e error
	q.Reason, e = AcceptanceText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	r := &q.Response
	r.Status, e = AcceptanceText("status", r.Status, 32, true)
	if e != nil {
		return q, e
	}
	if r.Status != "pending" && r.Status != "accepted" && r.Status != "accepted_with_exceptions" {
		return q, AcceptanceInvalid("status", "Choose pending, accepted or accepted with exceptions")
	}
	for _, v := range []struct {
		f        string
		p        *string
		n        int
		required bool
	}{{"signatory_name", &r.SignatoryName, 200, false}, {"document_reference", &r.DocumentReference, 300, false}, {"remarks", &r.Remarks, 4000, r.Status == "accepted_with_exceptions"}} {
		*v.p, e = AcceptanceText(v.f, *v.p, v.n, v.required)
		if e != nil {
			return q, e
		}
	}
	r.AcceptedDate, e = AcceptanceText("accepted_date", r.AcceptedDate, 10, false)
	if e != nil {
		return q, e
	}
	if r.Status == "pending" && r.AcceptedDate != "" {
		return q, AcceptanceInvalid("accepted_date", "A pending response has no vendor acceptance date")
	}
	if r.Status != "pending" && !AcceptanceDate(r.AcceptedDate) {
		return q, AcceptanceInvalid("accepted_date", "Provide the actual vendor acceptance date")
	}
	return q, nil
}
func AcceptanceApply(base *AcceptanceRecord, a Principal, caps AcceptanceAccess, q AcceptanceCommand, source AcceptanceSource, now AcceptanceTimestamp, id string) (AcceptanceRecord, error) {
	empty := AcceptanceRecord{}
	q, e := AcceptanceValidate(q)
	if e != nil {
		return empty, e
	}
	if e = AcceptanceAccessGuard(a, caps, true); e != nil {
		return empty, e
	}
	if e = source.Guard(); e != nil {
		return empty, e
	}
	if q.Response.Status != "pending" && (q.Response.AcceptedDate < source.Order.Date || q.Response.AcceptedDate > AcceptanceToday(now)) {
		return empty, AcceptanceInvalid("accepted_date", "Acceptance date must be between the issued PO date and today")
	}
	out := AcceptanceRecord{ID: id, ProjectID: source.ProjectID, PackageID: source.PackageID, Order: *source.Order, Active: true, CreatedAt: now, CreatedBy: a.ID}
	if base == nil {
		if q.Version != 0 {
			return empty, AcceptanceStale
		}
	} else {
		if !base.Active || base.ProjectID != source.ProjectID || base.PackageID != source.PackageID {
			return empty, AcceptanceMissing
		}
		if q.Version != base.Version || base.Order != *source.Order {
			return empty, AcceptanceStale
		}
		out = *base
	}
	if e = AcceptanceID(out.ID); e != nil {
		return empty, e
	}
	if e = AcceptanceID(a.ID); e != nil {
		return empty, e
	}
	out.Version = q.Version + 1
	out.Response = q.Response
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}

// AcceptedFact is recalculated by later consumers against the same released PO.
// Exceptions, pending responses and inactive records cannot grant clearance.
func AcceptanceAcceptedFact(r *AcceptanceRecord, source AcceptanceSource) (*AcceptanceRecord, error) {
	if e := source.Guard(); e != nil {
		return nil, e
	}
	if r == nil || !r.Active {
		return nil, AcceptanceMissing
	}
	if r.ProjectID != source.ProjectID || r.PackageID != source.PackageID || r.Order != *source.Order {
		return nil, AcceptanceStale
	}
	if r.Response.Status != "accepted" || !AcceptanceDate(r.Response.AcceptedDate) || r.Response.AcceptedDate < r.Order.Date || r.Version < 1 {
		return nil, AcceptanceInvalid("acceptance", "Vendor acceptance without unresolved exceptions is required")
	}
	out := *r
	return &out, nil
}
