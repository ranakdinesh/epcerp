package domain

// Progress records completion of the request task, not authority to approve or pay.
type AdvancesProgress struct {
	ID             string
	Version        int32
	State, Details string
	Current        bool
	Snapshot       *AdvancesRecord `json:"-"`
}

func AdvancesProgressFact(r *AdvancesRecord, expected AdvancesSourceFacts, upstreamCurrent bool, now AdvancesTimestamp) AdvancesProgress {
	out := AdvancesProgress{State: "not_started"}
	if r == nil || !r.Active {
		return out
	}
	out.ID, out.Version = r.ID, r.Version
	source := AdvancesSource{AdvancesSourceFacts: expected, Available: true}
	if !upstreamCurrent || source.Guard() != nil || r.ProjectID != expected.ProjectID || r.PackageID != expected.PackageID || r.Source.AdvancesSourceFacts != expected {
		out.State, out.Details = "blocked", "Advance evidence does not match the current order, acceptance and supplier verification"
		return out
	}
	out.Current = true
	if r.State == "draft" {
		return out
	}
	if r.State == "rejected" {
		out.State, out.Details = "blocked", "Finance rejected this advance request"
		return out
	}
	switch r.State {
	case "requested", "approved", "disbursed", "not_applicable":
	default:
		out.State, out.Details = "blocked", "Stored advance state is invalid"
		return out
	}
	if AdvancesFinanceReady(*r, source, AdvancesToday(now)) != nil {
		out.State, out.Details = "blocked", "Submitted advance evidence or exact amount is incomplete"
		return out
	}
	if r.State != "requested" && (AdvancesID(r.DecisionID) != nil || AdvancesID(r.ReviewedBy) != nil || r.ReviewedAt == nil || r.ReviewedAt.IsZero() || r.ReviewedBy == r.SubmittedBy) {
		out.State, out.Details = "blocked", "Actual independent Finance decision is missing"
		return out
	}
	if r.State == "not_applicable" && r.Details.Kind != "not_applicable" || (r.State == "approved" || r.State == "disbursed") && r.Details.Kind != "payment" {
		out.State, out.Details = "blocked", "Advance disposition does not match the Finance decision"
		return out
	}
	if r.State == "disbursed" && (r.Payment == nil || !AdvancesDate(r.Payment.Date) || r.Payment.Date < AdvancesToday(*r.ReviewedAt) || r.Payment.Date > AdvancesToday(now)) {
		out.State, out.Details = "blocked", "Actual payment evidence is missing"
		return out
	}
	if r.State == "disbursed" {
		if _, err := AdvancesText("voucher", r.Payment.Voucher, 200, true); err != nil {
			out.State, out.Details = "blocked", "Actual payment voucher is missing"
			return out
		}
	}
	out.State, out.Details = "completed", r.Details.RequestNo
	if r.State == "not_applicable" {
		out.State, out.Details = "not_applicable", r.Details.Notes
	}
	snapshot := AdvancesCopy(*r)
	out.Snapshot = &snapshot
	return out
}
