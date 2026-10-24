package domain

import (
	"testing"
)

func TestOwnerAndPortfolioAccess(t *testing.T) {
	p := Principal{ID: "owner", Active: true}
	if !p.Can("owner", "write") || p.Can("other", "read") {
		t.Fatal("owner boundaries")
	}
	p.Permissions = []string{"tenders.portfolio.read"}
	if !p.Can("other", "read") || p.Can("other", "write") {
		t.Fatal("portfolio read enabled write")
	}
	p.Superuser = true
	if !p.Can("other", "write") {
		t.Fatal("recovery authority")
	}
	p.Active = false
	if p.Allows("tenders.portfolio.read") || p.Can("owner", "write") {
		t.Fatal("inactive authority")
	}
}
