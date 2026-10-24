package domain

import (
	"fmt"
	"math/big"
	"regexp"
	"time"
)

var ordersDecimal = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,4})?$`)
var ordersCurrency = regexp.MustCompile(`^[A-Z]{3}$`)

func ordersValue(field, s string, precision int, positive bool) (*big.Rat, error) {
	if !ordersDecimal.MatchString(s) || len(s) > 22 {
		return nil, OrdersInvalid(field, "Use an exact nonnegative decimal")
	}
	var fraction int
	for i, c := range s {
		if c == '.' {
			fraction = len(s) - i - 1
		}
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok || fraction > precision || (positive && v.Sign() <= 0) {
		return nil, OrdersInvalid(field, "Use the permitted decimal precision and a positive quantity")
	}
	return v, nil
}
func ordersMoney(v *big.Rat) string {
	n := new(big.Int).Mul(v.Num(), big.NewInt(100))
	n.Mul(n, big.NewInt(2))
	n.Add(n, v.Denom())
	n.Quo(n, new(big.Int).Mul(v.Denom(), big.NewInt(2)))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, big.NewInt(100), r)
	return fmt.Sprintf("%s.%02d", q.String(), r.Int64())
}
func (s OrdersSource) Guard() error {
	if !s.Available || len(s.Blockers) > 0 {
		return OrdersInvalid("source", "Resolve current supplier, Finance, technical and project blockers before preparing or releasing the PO")
	}
	for _, v := range []string{s.ProjectID, s.PackageID, s.SourcingID, s.Supplier.ID, s.Technical.CaseID, s.Technical.ApprovalFileID, s.ItemID} {
		if e := OrdersID(v); e != nil {
			return e
		}
	}
	if s.SourcingVersion < 1 || s.Technical.Version < 1 || s.Technical.Revision < 1 || !s.Technical.QAPApproved || !s.Technical.ConditionsClosed || s.LOIReference == "" || s.FinancialReference == "" || s.SupplierVerification == "" || !OrdersDate(s.LOIDate) || !OrdersDate(s.Technical.ApprovalDate) || s.Technical.Reference == "" || s.Supplier.Name == "" || s.Supplier.Address == "" || s.Description == "" || s.UOM == "" || !ordersCurrency.MatchString(s.Currency) {
		return OrdersInvalid("source", "Actual released commercial, verified supplier and approved technical evidence is incomplete")
	}
	if s.DeliveryDays < 1 || s.DeliveryDays > 3650 || s.PaymentTerms == "" || s.Warranty == "" || s.LOITerms == "" {
		return OrdersInvalid("terms", "Actual approved delivery, payment, warranty and LOI terms are required")
	}
	q, e := ordersValue("quantity", s.Quantity, 4, true)
	if e != nil {
		return e
	}
	base, e := ordersValue("base", s.Base, 2, false)
	if e != nil {
		return e
	}
	if s.PricingMode == "unit_price" {
		r, e := ordersValue("unit_price", s.UnitPrice, 4, false)
		if e != nil {
			return e
		}
		if ordersMoney(new(big.Rat).Mul(q, r)) != ordersMoney(base) {
			return OrdersInvalid("base", "Exact approved unit rate and quantity do not match the basic amount")
		}
	} else if s.PricingMode != "line_total" || s.UnitPrice != "" {
		return OrdersInvalid("pricing_mode", "Use the exact approved unit-rate or direct-line-total pricing mode")
	}
	vals := []*big.Rat{}
	for _, r := range []struct{ f, s string }{{"discount", s.Discount}, {"tax", s.Tax}, {"freight", s.Freight}, {"other", s.Other}, {"total", s.Total}} {
		v, e := ordersValue(r.f, r.s, 2, false)
		if e != nil {
			return e
		}
		vals = append(vals, v)
	}
	if vals[0].Cmp(base) > 0 {
		return OrdersInvalid("discount", "Discount exceeds the approved basic value")
	}
	total := new(big.Rat).Sub(base, vals[0])
	for _, v := range vals[1:4] {
		total.Add(total, v)
	}
	if total.Cmp(vals[4]) != 0 {
		return OrdersInvalid("total", "Order amounts must equal the exact current Finance-approved total")
	}
	return nil
}
func OrdersSameSource(a, b OrdersSource) bool { return a.OrdersSourceFacts == b.OrdersSourceFacts }
func OrdersAccessGuard(a Principal, c OrdersAccess, action string) error {
	if !a.Active || a.ID == "" {
		return OrdersUnauthorized
	}
	if !c.Read {
		return OrdersForbidden
	}
	if action == "read" {
		return nil
	}
	if !c.Work || !c.Write || (action == "release" && !c.Release) || (action == "client_decision" && !c.RecordClient) {
		return OrdersForbidden
	}
	return nil
}
func OrdersValidate(q OrdersCommand) (OrdersCommand, error) {
	var e error
	q.Reason, e = OrdersText("reason", q.Reason, 2000, true)
	if e != nil {
		return q, e
	}
	if q.Version < 0 {
		return q, OrdersInvalid("version", "Use the current nonnegative version")
	}
	if q.Action != "prepare" && q.Action != "release" && q.Action != "submit_client" && q.Action != "client_decision" {
		return q, OrdersInvalid("action", "Choose prepare, submit_client, client_decision or release")
	}
	if (q.Action == "prepare") != (q.Details != nil) {
		return q, OrdersInvalid("details", "Only preparation accepts complete order details")
	}
	if (q.Action == "submit_client") != (q.Submission != nil) {
		return q, OrdersInvalid("submission", "Only client submission accepts its reference and date")
	}
	if (q.Action == "client_decision") != (q.ClientDecision != nil) {
		return q, OrdersInvalid("client_decision", "Only client decision accepts signed approval evidence")
	}
	return q, nil
}
func ordersValidateDetails(d OrdersDetails, s OrdersSource, today string) (OrdersDetails, error) {
	for _, v := range []struct {
		f        string
		p        *string
		n        int
		required bool
	}{{"issuer_name", &d.IssuerName, 200, true}, {"issuer_address", &d.IssuerAddress, 2000, true}, {"issuer_tax_reference", &d.IssuerTaxReference, 80, false}, {"issuer_registration", &d.IssuerRegistration, 120, false}, {"issuer_email", &d.IssuerEmail, 254, false}, {"bill_to", &d.BillTo, 2000, true}, {"delivery_address", &d.DeliveryAddress, 2000, true}} {
		var e error
		*v.p, e = OrdersText(v.f, *v.p, v.n, v.required)
		if e != nil {
			return d, e
		}
	}
	if d.IssuerEmail != "" {
		if !regexp.MustCompile(`^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$`).MatchString(d.IssuerEmail) {
			return d, OrdersInvalid("issuer_email", "Use a valid email address")
		}
	}
	if !OrdersDate(d.Date) || d.Date < s.LOIDate || d.Date < s.Technical.ApprovalDate || d.Date > today {
		return d, OrdersInvalid("date", "PO date must be between LOI/client approval and today")
	}
	return d, nil
}
func OrdersFiscalYear(date string) (int, error) {
	if !OrdersDate(date) {
		return 0, OrdersInvalid("date", "Use a valid PO date")
	}
	t, _ := time.Parse("2006-01-02", date)
	year := t.Year()
	if t.Month() < time.April {
		year--
	}
	return year, nil
}
func OrdersNumber(date string, sequence int64) (string, error) {
	year, e := OrdersFiscalYear(date)
	if e != nil {
		return "", e
	}
	if sequence < 1 || sequence > 999999999 {
		return "", OrdersInvalid("number", "Purchase-order sequence exceeds its supported range")
	}
	return fmt.Sprintf("PO/VTP/%04d-%02d/%06d", year, (year+1)%100, sequence), nil
}
func OrdersApply(base *OrdersOrder, a Principal, c OrdersAccess, q OrdersCommand, s OrdersSource, now OrdersTimestamp, id, number string) (OrdersOrder, error) {
	empty := OrdersOrder{}
	var e error
	q, e = OrdersValidate(q)
	if e != nil {
		return empty, e
	}
	if e = OrdersAccessGuard(a, c, q.Action); e != nil {
		return empty, e
	}
	if e = s.Guard(); e != nil {
		return empty, e
	}
	version := int32(0)
	if base != nil {
		version = base.Version
		if !base.Active {
			return empty, OrdersMissing
		}
		if base.State != "draft" {
			return empty, OrdersInvalid("state", "Released orders are immutable; use a governed amendment workflow")
		}
	}
	if q.Version != version {
		return empty, OrdersStale
	}
	out := OrdersOrder{ID: id, ProjectID: s.ProjectID, PackageID: s.PackageID, Version: 1, State: "draft", Active: true, CreatedAt: now, CreatedBy: a.ID}
	if base != nil {
		out = OrdersCopy(*base)
		out.Version++
	}
	if q.Action == "prepare" {
		out.Details, e = ordersValidateDetails(*q.Details, s, OrdersToday(now))
		if e != nil {
			return empty, e
		}
		out.DraftRevision++
		out.ClientApproval = OrdersClientApproval{Status: "not_submitted", DraftRevision: out.DraftRevision}
		out.Source = s
		out.Source.Blockers = append([]string{}, s.Blockers...)
	} else {
		if base == nil {
			return empty, OrdersInvalid("order", "Prepare the complete order before client submission or release")
		}
		if !OrdersSameSource(out.Source, s) {
			return empty, OrdersStale
		}
		if _, e = ordersValidateDetails(out.Details, s, OrdersToday(now)); e != nil {
			return empty, e
		}
		switch q.Action {
		case "submit_client":
			if out.DraftRevision < 1 {
				return empty, OrdersInvalid("order", "Review and save the existing draft before client submission")
			}
			if out.ClientApproval.Status == "submitted" || out.ClientApproval.Status == "approved" {
				return empty, OrdersInvalid("state", "Revise the draft before submitting it again")
			}
			submission := *q.Submission
			submission.Reference, e = OrdersText("reference", submission.Reference, 200, true)
			if e != nil {
				return empty, e
			}
			if !OrdersDate(submission.Date) || submission.Date < out.Details.Date || submission.Date > OrdersToday(now) {
				return empty, OrdersInvalid("date", "Submission date must be between draft date and today")
			}
			out.ClientApproval = OrdersClientApproval{Status: "submitted", DraftRevision: out.DraftRevision, Submission: &submission, SubmittedBy: a.ID}
		case "client_decision":
			if out.ClientApproval.Status != "submitted" || out.ClientApproval.DraftRevision != out.DraftRevision {
				return empty, OrdersInvalid("state", "Submit this exact draft to the client before recording a decision")
			}
			d := *q.ClientDecision
			if d.Status != "approved" && d.Status != "rejected" {
				return empty, OrdersInvalid("status", "Choose approved or rejected")
			}
			for _, v := range []struct {
				f string
				p *string
				n int
			}{{"reference", &d.Reference, 200}, {"signatory", &d.Signatory, 200}, {"designation", &d.Designation, 200}} {
				*v.p, e = OrdersText(v.f, *v.p, v.n, true)
				if e != nil {
					return empty, e
				}
			}
			if !OrdersDate(d.Date) || d.Date < out.ClientApproval.Submission.Date || d.Date > OrdersToday(now) {
				return empty, OrdersInvalid("date", "Decision date must be between submission and today")
			}
			if d.Status == "approved" {
				if e = OrdersID(d.FileID); e != nil {
					return empty, OrdersInvalid("file_id", "Upload the client stamped and signed PO")
				}
			} else if d.FileID != "" {
				if e = OrdersID(d.FileID); e != nil {
					return empty, e
				}
			}
			out.ClientApproval.Status = d.Status
			out.ClientApproval.Decision = &d
			out.ClientApproval.RecordedBy = a.ID
			out.ClientApproval.RecordedAt = &now
		case "release":
			if !OrdersClientCleared(out) {
				return empty, OrdersInvalid("client_approval", "Record client stamp/sign approval for the current draft before PO release")
			}
			if number == "" {
				return empty, OrdersInvalid("number", "Allocate an owned PO number before release")
			}
		}
		if q.Action != "release" {
			out.UpdatedAt = now
			out.UpdatedBy = a.ID
			return out, nil
		}
		out.State = "released"
		out.Number = number
		out.ReleasedBy = a.ID
		out.ReleasedAt = &now
	}
	out.UpdatedAt = now
	out.UpdatedBy = a.ID
	return out, nil
}

// BasicAmount materializes the approved rate without losing four-decimal rate
// precision. Direct line totals remain authoritative without an invented rate.
func OrdersBasicAmount(mode, rate, lineTotal, quantity string) (string, error) {
	if mode == "line_total" {
		v, e := ordersValue("base", lineTotal, 2, false)
		if e != nil {
			return "", e
		}
		return ordersMoney(v), nil
	}
	if mode != "unit_price" && mode != "" {
		return "", OrdersInvalid("pricing_mode", "Unsupported approved price mode")
	}
	r, e := ordersValue("unit_price", rate, 4, false)
	if e != nil {
		return "", e
	}
	q, e := ordersValue("quantity", quantity, 4, true)
	if e != nil {
		return "", e
	}
	return ordersMoney(new(big.Rat).Mul(q, r)), nil
}
