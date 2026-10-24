// epcERP finance role presets. Existing administrator-managed roles are preserved.
package application

import (
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func FinancePresets() []domain.RoleCreate {
	return []domain.RoleCreate{
		{Name: "finance.viewer", Description: "Finance level 1: read assigned projects, budgets and quotation variances", PermissionCodes: []string{"communication.inbox.read", "work.tasks.read", "work.tasks.view", "accounts.quotation_approvals.read", "accounts.quotation_approvals.view", "projects.budgets.read", "projects.budgets.view", "projects.dashboard.read", "projects.dashboard.view", "projects.group_comparisons.read", "projects.group_comparisons.view", "projects.master.read", "projects.master.view", "projects.purchase_bo.read", "projects.purchase_bo.view"}},
		{Name: "finance.editor", Description: "Finance level 2: prepare and revise assigned project/item budgets", PermissionCodes: []string{"communication.inbox.read", "work.tasks.read", "work.tasks.view", "accounts.quotation_approvals.read", "accounts.quotation_approvals.view", "projects.budgets.read", "projects.budgets.view", "projects.budgets.write", "projects.dashboard.read", "projects.dashboard.view", "projects.group_comparisons.read", "projects.group_comparisons.view", "projects.master.read", "projects.master.view", "projects.purchase_bo.read", "projects.purchase_bo.view"}},
		{Name: "finance.approver", Description: "Finance level 3: approve budgets, verify vendors and decide Accounts reviews", PermissionCodes: []string{"communication.inbox.read", "work.tasks.read", "work.tasks.view", "accounts.quotation_approvals.read", "accounts.quotation_approvals.view", "accounts.quotation_approvals.write", "accounts.vendor_verification.read", "accounts.vendor_verification.view", "accounts.vendor_verification.write", "projects.budgets.approve", "projects.budgets.read", "projects.budgets.view", "projects.budgets.write", "projects.dashboard.read", "projects.dashboard.view", "projects.group_comparisons.read", "projects.group_comparisons.view", "projects.master.read", "projects.master.view", "projects.purchase_bo.read", "projects.purchase_bo.view"}},
	}
}
