package application

import (
	"context"
	"testing"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type referenceRepository struct{ calls int }

func (r *referenceRepository) Get(context.Context, string, string) (*masterDomain.Reference, error) {
	r.calls++
	return &masterDomain.Reference{ID: "id", Active: true}, nil
}
func TestReferenceReadContractRejectsUnknownKind(t *testing.T) {
	r := &referenceRepository{}
	s := NewReferenceService(r)
	if _, e := s.Get(context.Background(), "users", "id"); e == nil || r.calls != 0 {
		t.Fatal("unknown kind reached persistence")
	}
	v, e := s.Get(context.Background(), "cities", "id")
	if e != nil || v.ID != "id" || r.calls != 1 {
		t.Fatal(v, e)
	}
}
