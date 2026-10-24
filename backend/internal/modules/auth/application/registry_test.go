package application

import (
	"testing"
)

func TestOwnedRegistryAndPresets(t *testing.T) {
	groups := map[string]bool{"settings": true, "projects": true, "purchasing": true, "tenders": true, "accounts": true, "administration": true, "documents": true, "system": true}
	codes := map[string]bool{}
	for _, p := range Registry() {
		if p.Code == "" || p.Description == "" || !groups[p.Group] || codes[p.Code] {
			t.Fatalf("invalid permission definition: %s", p.Code)
		}
		codes[p.Code] = true
	}
	for _, preset := range FinancePresets() {
		for _, code := range preset.PermissionCodes {
			if !codes[code] {
				t.Fatalf("preset references unregistered permission: %s", code)
			}
		}
	}
	for _, code := range []string{"settings.clients.read", "settings.manufacturers.write", "documents.storage.read"} {
		if !codes[code] {
			t.Fatalf("missing implemented capability permission: %s", code)
		}
	}
}

func TestQualityRegistryUsesPurchasingGroup(t *testing.T) {
	for _, code := range []string{"projects.purchase_quality.read", "projects.purchase_quality.write", "projects.purchase_quality.verify"} {
		found := false
		for _, p := range Registry() {
			if p.Code == code {
				found = true
				if p.Group != "purchasing" || p.Description == "" {
					t.Fatal("quality permission grouping", code)
				}
			}
		}
		if !found {
			t.Fatal("missing quality permission", code)
		}
	}
}

func TestClientClearanceRegistryUsesPurchasingGroup(t *testing.T) {
	for _, code := range []string{"projects.purchase_client_clearance.read", "projects.purchase_client_clearance.write"} {
		found := false
		for _, p := range Registry() {
			if p.Code == code {
				found = true
				if p.Group != "purchasing" || p.Description == "" {
					t.Fatal("MDCC permission grouping", code)
				}
			}
		}
		if !found {
			t.Fatal("missing MDCC permission", code)
		}
	}
}

func TestDispatchRegistryUsesPurchasingGroup(t *testing.T) {
	for _, code := range []string{"projects.purchase_engineer_confirmations.read", "projects.purchase_engineer_confirmations.write", "projects.purchase_grns.read", "projects.purchase_grns.write", "projects.purchase_site_receipts.read", "projects.purchase_site_receipts.write", "projects.purchase_dispatch.read", "projects.purchase_dispatch.write"} {
		found := false
		for _, p := range Registry() {
			if p.Code == code {
				found = true
				if p.Group != "purchasing" || p.Description == "" {
					t.Fatal("dispatch permission grouping", code)
				}
			}
		}
		if !found {
			t.Fatal("missing dispatch permission", code)
		}
	}
}

func TestAccountsAcceptanceRegistryUsesAccountsGroup(t *testing.T) {
	for _, code := range []string{"accounts.purchase_acceptance.view", "accounts.purchase_acceptance.read", "accounts.purchase_acceptance.write"} {
		found := false
		for _, p := range Registry() {
			if p.Code == code {
				found = true
				if p.Group != "accounts" || p.Description == "" {
					t.Fatal("Accounts grouping", code)
				}
			}
		}
		if !found {
			t.Fatal("missing Accounts permission", code)
		}
	}
}
