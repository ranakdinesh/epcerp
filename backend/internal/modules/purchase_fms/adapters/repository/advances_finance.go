package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	finance "github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type AdvancesFinanceBridge struct{ f finance.AdvanceReviewer }

func NewAdvancesFinanceBridge(f finance.AdvanceReviewer) *AdvancesFinanceBridge {
	return &AdvancesFinanceBridge{f}
}
func advancesSubmission(r domain.AdvancesRecord) (string, string, error) {
	source := struct {
		Source              domain.AdvancesSourceFacts
		Details             domain.AdvancesDetails
		Amount, SubmittedBy string
		SubmittedVersion    int32
		SubmittedAt         *domain.AdvancesTimestamp
	}{r.Source.AdvancesSourceFacts, r.Details, r.Amount, r.SubmittedBy, r.SubmittedVersion, r.SubmittedAt}
	raw, e := json.Marshal(source)
	if e != nil {
		return "", "", e
	}
	sum := sha256.Sum256(raw)
	return string(raw), hex.EncodeToString(sum[:]), nil
}
func (b *AdvancesFinanceBridge) Decide(ctx context.Context, a domain.Principal, r domain.AdvancesRecord, outcome, reason string) (string, error) {
	_, fingerprint, e := advancesSubmission(r)
	if e != nil {
		return "", e
	}
	id, e := b.f.ReviewAdvance(ctx, finance.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, finance.AdvanceRequest{ProjectID: r.ProjectID, PackageID: r.PackageID, RequestID: r.ID, OrderID: r.Source.OrderID, SubmittedBy: r.SubmittedBy, Outcome: outcome, Amount: r.Amount, Currency: r.Source.Currency, Fingerprint: fingerprint, Reason: reason, Version: r.SubmittedVersion, OrderVersion: r.Source.OrderVersion, Kind: r.Details.Kind})
	var d *finance.Error
	if errors.As(e, &d) {
		return "", &domain.AdvancesError{Status: d.Status, Field: d.Field, Message: d.Message}
	}
	return id, e
}
