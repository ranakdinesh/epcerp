package domain

// Issued orders are immutable historical commitments. Reporting an issued order
// does not re-authorize release or retroactively withdraw it when live gates change.
type OrdersProgress struct {
	ID             string
	Version        int32
	State, Details string
	Current        bool
	Snapshot       *OrdersOrder `json:"-"`
}

func OrdersProgressFact(record *OrdersOrder, project, pkg string) OrdersProgress {
	out := OrdersProgress{State: "not_started"}
	if record == nil || !record.Active {
		return out
	}
	out.ID = record.ID
	out.Version = record.Version
	if record.ProjectID != project || record.PackageID != pkg || record.Source.ProjectID != project || record.Source.PackageID != pkg {
		out.State = "blocked"
		out.Details = "Recorded order does not match the reporting scope"
		return out
	}
	if record.State == "draft" {
		out.State = "in_progress"
		out.Details = "Purchase order draft awaits authorized release"
		return out
	}
	released, e := OrdersReleasedFact(record, project, pkg)
	if e != nil {
		out.State = "blocked"
		out.Details = "Recorded release evidence is incomplete"
		return out
	}
	out.State = "completed"
	out.Current = true
	out.Details = released.Number
	out.Snapshot = released
	return out
}
