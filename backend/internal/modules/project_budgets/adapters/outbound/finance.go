package outbound

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	finance "github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/ports"
)

type FinanceBridge struct{ p finance.BudgetApprover }

func NewFinanceBridge(p finance.BudgetApprover) ports.Approvals { return &FinanceBridge{p} }
func (f *FinanceBridge) DecideBudget(ctx context.Context, a domain.Actor, b domain.Budget, reason string) (string, error) {
	data, e := json.Marshal(b.Data)
	if e != nil {
		return "", e
	}
	hash := sha256.Sum256(data)
	id, e := f.p.ApproveBudget(ctx, finance.Actor{ID: a.ID, Active: a.Active, Superuser: a.Superuser, Permissions: a.Permissions}, finance.BudgetRequest{ProjectID: b.ProjectID, Version: b.Version, Revision: b.Revision, Amount: b.Data.Total, Currency: b.Data.Currency, Fingerprint: hex.EncodeToString(hash[:]), Reason: reason})
	var d *finance.Error
	if errors.As(e, &d) {
		return "", &domain.Error{Status: d.Status, Field: d.Field, Message: d.Message}
	}
	return id, e
}
