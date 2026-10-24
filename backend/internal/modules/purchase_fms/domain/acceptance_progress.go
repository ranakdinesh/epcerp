package domain

// Vendor acknowledgement is recorded work. Accepted-with-exceptions is reported
// as acknowledgement complete while WithoutExceptions stays false for downstream execution.
type AcceptanceProgress struct {
	ID                         string
	Version                    int32
	State, Details             string
	Current, WithoutExceptions bool
	Snapshot                   *AcceptanceRecord `json:"-"`
}

func AcceptanceProgressFact(r *AcceptanceRecord, current AcceptanceSource) AcceptanceProgress {
	out := AcceptanceProgress{State: "not_started"}
	if r == nil || !r.Active {
		return out
	}
	out.ID = r.ID
	out.Version = r.Version
	if current.Order == nil || r.ProjectID != current.ProjectID || r.PackageID != current.PackageID || r.Order != *current.Order {
		out.State = "blocked"
		out.Details = "Vendor response does not match the current released order"
		return out
	}
	out.Current = true
	if r.Response.Status == "pending" {
		return out
	}
	_, e := AcceptanceValidate(AcceptanceCommand{Version: r.Version, Action: "record", Reason: "Validate stored reporting evidence", Response: r.Response})
	if e != nil || r.Response.AcceptedDate < r.Order.Date {
		out.State = "blocked"
		out.Details = "Recorded vendor acknowledgement evidence is incomplete"
		return out
	}
	snapshot := *r
	out.Snapshot = &snapshot
	out.State = "completed"
	out.WithoutExceptions = r.Response.Status == "accepted"
	out.Details = r.Response.DocumentReference
	if !out.WithoutExceptions {
		out.Details = "Acknowledged with exceptions: " + r.Response.Remarks
	}
	return out
}
