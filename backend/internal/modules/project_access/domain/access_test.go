package domain

import (
	"testing"
)

func TestScopedProfilesNeverGrantGlobalOrOwningAuthority(t *testing.T) {
	for _, code := range []string{"system:admin", "admin.users.write", "settings.clients.write", "accounts.vendor_verification.write", "projects.portfolio.write", "projects.purchase_portfolio.write", "projects.master.write", "projects.manager_assignments.write", "projects.purchase_assignments.write", "projects.team.write", "projects.processes.bypass"} {
		if ScopedCode(code) {
			t.Fatalf("global/owning code accepted: %s", code)
		}
	}
	for _, code := range []string{"projects.budgets.read", "projects.budgets.approve", "accounts.quotation_approvals.write", "accounts.purchase_advances.read", "accounts.purchase_advances.write", "accounts.purchase_advances.disburse", "projects.team.read", "projects.planning.write"} {
		if !ScopedCode(code) {
			t.Fatalf("scoped code rejected: %s", code)
		}
	}
}
func TestEffectiveProfileRequiresLiveIntersection(t *testing.T) {
	const code = "projects.budgets.approve"
	a := Actor{ID: "user", Active: true, Permissions: []string{code}}
	u := User{ID: "user", Active: true, Permissions: []string{code}}
	b := Bundle{ID: "role", Active: true, Permissions: []string{code}}
	p := Profile{UserID: "user", BundleID: "role", Active: true, Permissions: []string{code}}
	if !Effective(a, u, b, true, p, code) {
		t.Fatal("eligible profile denied")
	}
	tests := []struct {
		name   string
		change func(*Actor, *User, *Bundle, *Profile)
	}{
		{"inactive actor", func(a *Actor, u *User, b *Bundle, p *Profile) { a.Active = false }},
		{"inactive user", func(a *Actor, u *User, b *Bundle, p *Profile) { u.Active = false }},
		{"revoked role", func(a *Actor, u *User, b *Bundle, p *Profile) { b.Active = false }},
		{"revoked profile", func(a *Actor, u *User, b *Bundle, p *Profile) { p.Active = false }},
		{"role code removed", func(a *Actor, u *User, b *Bundle, p *Profile) { b.Permissions = nil }},
		{"global eligibility removed", func(a *Actor, u *User, b *Bundle, p *Profile) { u.Permissions = nil }},
		{"different user", func(a *Actor, u *User, b *Bundle, p *Profile) { a.ID = "other" }},
		{"different role", func(a *Actor, u *User, b *Bundle, p *Profile) { p.BundleID = "other" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			aa, uu, bb, pp := a, u, b, p
			tc.change(&aa, &uu, &bb, &pp)
			if Effective(aa, uu, bb, true, pp, code) {
				t.Fatal("revocation bypassed")
			}
		})
	}
	if Effective(a, u, b, false, p, code) {
		t.Fatal("role membership removed but still effective")
	}
}

func TestAdvanceFinancialActionsRequireExplicitFinanceScope(t *testing.T) {
	for _, code := range []string{"accounts.purchase_advances.write", "accounts.purchase_advances.disburse"} {
		if !FinancialDecision(code) {
			t.Fatalf("Financial action inherits automatic PM authority: %s", code)
		}
	}
	if FinancialDecision("accounts.purchase_advances.read") {
		t.Fatal("Read authority classified as a decision")
	}
}

func TestAccountsAcceptanceRequiresScopedFinancialAuthority(t *testing.T) {
	if !ScopedCode("accounts.purchase_acceptance.read") || !ScopedCode("accounts.purchase_acceptance.write") || !FinancialDecision("accounts.purchase_acceptance.write") || FinancialDecision("accounts.purchase_acceptance.read") {
		t.Fatal("Accounts scope classification")
	}
}
