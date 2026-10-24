package domain

import (
	"fmt"
	"math/big"
	"regexp"
)

var advancesDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,13})(\.[0-9]{1,2})?$`)
var advancesCurrency = regexp.MustCompile(`^[A-Z]{3}$`)

func advancesExact(field, s string) (*big.Rat, error) {
	if !advancesDecimal.MatchString(s) {
		return nil, AdvancesInvalid(field, "Use a plain exact nonnegative decimal with at most two places")
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, AdvancesInvalid(field, "Invalid exact decimal")
	}
	return v, nil
}
func advancesMoney(v *big.Rat) string {
	n := new(big.Int).Mul(v.Num(), big.NewInt(200))
	n.Add(n, v.Denom())
	n.Quo(n, new(big.Int).Mul(v.Denom(), big.NewInt(2)))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, big.NewInt(100), r)
	return fmt.Sprintf("%s.%02d", q.String(), r.Int64())
}
func AdvancesAmount(total, pct string) (string, error) {
	t, e := advancesExact("source", total)
	if e != nil {
		return "", e
	}
	p, e := advancesExact("advance_pct", pct)
	if e != nil {
		return "", e
	}
	if p.Cmp(big.NewRat(100, 1)) > 0 {
		return "", AdvancesInvalid("advance_pct", "Agreed advance percentage cannot exceed 100")
	}
	return advancesMoney(new(big.Rat).Quo(new(big.Rat).Mul(t, p), big.NewRat(100, 1))), nil
}
func (s AdvancesSource) Guard() error {
	if !s.Available || len(s.Blockers) > 0 {
		return AdvancesInvalid("source", "Resolve current project, vendor and unconditional acceptance blockers first")
	}
	for _, v := range []string{s.ProjectID, s.PackageID, s.OrderID, s.SupplierID, s.AcceptanceID} {
		if e := AdvancesID(v); e != nil {
			return e
		}
	}
	t, e := advancesExact("source", s.Total)
	if e != nil {
		return e
	}
	if t.Sign() < 0 || s.OrderVersion < 2 || s.AcceptanceVersion < 1 || s.Number == "" || !AdvancesDate(s.Date) || s.SupplierName == "" || !advancesCurrency.MatchString(s.Currency) || s.PaymentTerms == "" || s.SupplierVerification == "" || s.CalculationBasis != "po_total" {
		return AdvancesInvalid("source", "Use the actual released PO, its agreed terms and current accepted vendor evidence")
	}
	return nil
}
func AdvancesAccessGuard(a Principal, c AdvancesAccess, action string) error {
	if !a.Active || a.ID == "" {
		return AdvancesUnauthorized
	}
	if !c.Read {
		return AdvancesForbidden
	}
	switch action {
	case "read":
		return nil
	case "prepare", "submit", "revise":
		if !c.Write || !c.Work {
			return AdvancesForbidden
		}
	case "approve", "reject":
		if !c.Review {
			return AdvancesForbidden
		}
	case "disburse":
		if !c.Disburse {
			return AdvancesForbidden
		}
	default:
		return AdvancesInvalid("action", "Choose a declared advance action")
	}
	return nil
}
func AdvancesValidate(q AdvancesCommand) (AdvancesCommand, error) {
	if q.Version < 0 {
		return q, AdvancesInvalid("version", "Use the current nonnegative version")
	}
	var e error
	q.Reason, e = AdvancesText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	switch q.Action {
	case "prepare":
		if q.Details == nil || q.Payment != nil {
			return q, AdvancesInvalid("details", "Only preparation accepts complete request details")
		}
	case "disburse":
		if q.Details != nil || q.Payment == nil {
			return q, AdvancesInvalid("payment", "Disbursement accepts only payment evidence")
		}
	case "submit", "revise", "approve", "reject":
		if q.Details != nil || q.Payment != nil {
			return q, AdvancesInvalid("body", "This action accepts no request changes or payment flags")
		}
	default:
		return q, AdvancesInvalid("action", "Choose a declared advance action")
	}
	return q, nil
}
func advancesValidateDetails(d AdvancesDetails, s AdvancesSource, today string, submission bool) (AdvancesDetails, string, error) {
	if d.Kind != "payment" && d.Kind != "not_applicable" {
		return d, "", AdvancesInvalid("kind", "Choose a payment request or explicit no-advance disposition")
	}
	for _, v := range []struct {
		f        string
		p        *string
		n        int
		required bool
	}{{"request_no", &d.RequestNo, 120, submission}, {"proforma_invoice_ref", &d.InvoiceReference, 200, submission && d.Kind == "payment"}, {"bank_account_ref", &d.BankReference, 500, submission && d.Kind == "payment"}, {"notes", &d.Notes, 4000, submission && d.Kind == "not_applicable"}} {
		var e error
		*v.p, e = AdvancesText(v.f, *v.p, v.n, v.required)
		if e != nil {
			return d, "", e
		}
	}
	for _, v := range []struct {
		f        string
		s        string
		required bool
	}{{"request_date", d.RequestDate, submission}, {"proforma_invoice_date", d.InvoiceDate, submission && d.Kind == "payment"}} {
		if v.s != "" && (!AdvancesDate(v.s) || v.f == "request_date" && v.s < s.Date || v.s > today) || v.required && v.s == "" {
			return d, "", AdvancesInvalid(v.f, "Use an actual date between the issued PO and today")
		}
	}
	if d.InvoiceDate != "" && d.RequestDate != "" && d.InvoiceDate > d.RequestDate {
		return d, "", AdvancesInvalid("proforma_invoice_date", "Invoice date cannot follow the advance request date")
	}
	if d.Percentage == "" && !submission {
		return d, "", nil
	}
	amount, e := AdvancesAmount(s.Total, d.Percentage)
	if e != nil {
		return d, "", e
	}
	if d.Kind == "not_applicable" {
		percent, _ := advancesExact("advance_pct", d.Percentage)
		if amount != "0.00" || percent.Sign() != 0 {
			return d, "", AdvancesInvalid("advance_pct", "No-advance disposition requires an explicit zero percentage")
		}
		if d.InvoiceReference != "" || d.InvoiceDate != "" || d.BankReference != "" {
			return d, "", AdvancesInvalid("kind", "No-advance disposition carries no invoice or bank instructions")
		}
	} else if submission && amount == "0.00" {
		return d, "", AdvancesInvalid("advance_pct", "Submit a positive advance or choose not applicable")
	}
	return d, amount, nil
}
func AdvancesCan(r *AdvancesRecord, a Principal, c AdvancesAccess, action string, s AdvancesSource) error {
	if e := AdvancesAccessGuard(a, c, action); e != nil {
		return e
	}
	if action == "read" {
		return nil
	}
	if e := s.Guard(); e != nil {
		return e
	}
	if r == nil {
		if action != "prepare" {
			return AdvancesInvalid("request", "Prepare the saved request first")
		}
		return nil
	}
	if !r.Active || r.ProjectID != s.ProjectID || r.PackageID != s.PackageID {
		return AdvancesMissing
	}
	if r.State == "disbursed" {
		return AdvancesInvalid("state", "Disbursed advances are immutable; use a separately evidenced reversal workflow")
	}
	switch action {
	case "prepare":
		if r.State != "draft" {
			return AdvancesInvalid("state", "Revise the submitted request before changing its details")
		}
	case "submit":
		if r.State != "draft" {
			return AdvancesInvalid("state", "Only a complete draft can be submitted")
		}
	case "revise":
		if r.State == "draft" {
			return AdvancesInvalid("state", "The request is already a draft")
		}
	case "approve", "reject":
		if r.State != "requested" || a.ID == r.SubmittedBy {
			return AdvancesForbidden
		}
	case "disburse":
		if r.State != "approved" || r.DecisionID == "" || r.ReviewedAt == nil || r.Details.Kind != "payment" {
			return AdvancesInvalid("state", "Record payment only against an exact Finance-approved payment request")
		}
	}
	if action != "prepare" && action != "revise" && r.Source.AdvancesSourceFacts != s.AdvancesSourceFacts {
		return AdvancesStale
	}
	return nil
}
func AdvancesApply(base *AdvancesRecord, a Principal, c AdvancesAccess, q AdvancesCommand, s AdvancesSource, now AdvancesTimestamp, id, decision string) (AdvancesRecord, error) {
	var empty AdvancesRecord
	q, e := AdvancesValidate(q)
	if e != nil {
		return empty, e
	}
	if e = AdvancesCan(base, a, c, q.Action, s); e != nil {
		return empty, e
	}
	out := AdvancesRecord{ID: id, ProjectID: s.ProjectID, PackageID: s.PackageID, State: "draft", Active: true, Source: s, CreatedAt: now, CreatedBy: a.ID}
	if base == nil {
		if q.Version != 0 {
			return empty, AdvancesStale
		}
	} else {
		if q.Version != base.Version {
			return empty, AdvancesStale
		}
		out = AdvancesCopy(*base)
	}
	if e = AdvancesID(out.ID); e != nil {
		return empty, e
	}
	if e = AdvancesID(a.ID); e != nil {
		return empty, e
	}
	switch q.Action {
	case "prepare":
		out.Source = s
		out.Details, out.Amount, e = advancesValidateDetails(*q.Details, s, AdvancesToday(now), false)
	case "submit":
		out.Details, out.Amount, e = advancesValidateDetails(out.Details, s, AdvancesToday(now), true)
		out.State = "requested"
		out.SubmittedBy = a.ID
		out.SubmittedAt = &now
		out.SubmittedVersion = q.Version + 1
	case "revise":
		out.State = "draft"
		out.Source = s
		out.DecisionID = ""
		out.ReviewedBy = ""
		out.ReviewedAt = nil
		out.SubmittedBy = ""
		out.SubmittedAt = nil
		out.SubmittedVersion = 0
	case "approve", "reject":
		if e = AdvancesFinanceReady(out, s, AdvancesToday(now)); e != nil {
			return empty, e
		}
		if e = AdvancesID(decision); e != nil {
			return empty, e
		}
		out.DecisionID = decision
		out.ReviewedBy = a.ID
		out.ReviewedAt = &now
		if q.Action == "reject" {
			out.State = "rejected"
		} else if out.Details.Kind == "not_applicable" {
			out.State = "not_applicable"
		} else {
			out.State = "approved"
		}
	case "disburse":
		if e = AdvancesFinanceReady(out, s, AdvancesToday(now)); e != nil {
			return empty, e
		}
		p := *q.Payment
		p.Voucher, e = AdvancesText("voucher_ref", p.Voucher, 200, true)
		if e != nil {
			return empty, e
		}
		p.Notes, e = AdvancesText("notes", p.Notes, 4000, false)
		if e != nil {
			return empty, e
		}
		if !AdvancesDate(p.Date) || p.Date < AdvancesToday(*out.ReviewedAt) || p.Date > AdvancesToday(now) {
			return empty, AdvancesInvalid("disbursed_date", "Payment date must be between Finance approval and today")
		}
		out.Payment = &p
		out.State = "disbursed"
	}
	if e != nil {
		return empty, e
	}
	out.Version = q.Version + 1
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}

// Validate immutable submitted evidence before asking Finance to decide.
func AdvancesFinanceReady(r AdvancesRecord, s AdvancesSource, today string) error {
	_, calculated, e := advancesValidateDetails(r.Details, s, today, true)
	if e != nil {
		return e
	}
	if calculated != r.Amount || r.SubmittedAt == nil || r.SubmittedAt.IsZero() || r.SubmittedVersion < 2 {
		return AdvancesStale
	}
	return AdvancesID(r.SubmittedBy)
}
