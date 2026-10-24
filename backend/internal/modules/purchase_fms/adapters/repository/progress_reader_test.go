package repository

import (
	"testing"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

func TestProgressUnboundOrUnscopedReadCannotExposeSource(t *testing.T) {
	r := &progressReader{}
	if e := r.Authorize(t.Context(), domain.Principal{ID: "actor", Active: true}, "11111111-1111-4111-8111-111111111111"); e == nil {
		t.Fatal("unbound authority")
	}
	if _, e := r.Source(t.Context(), "11111111-1111-4111-8111-111111111111"); e != domain.ProgressForbidden {
		t.Fatal("unscoped source")
	}
}
