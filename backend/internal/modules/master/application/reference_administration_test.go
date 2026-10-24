package application

import (
	"context"
	"errors"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
	"testing"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type referenceFixture struct {
	records                 map[string]masterDomain.Reference
	locked                  bool
	units, writes, cascades int
	permission              string
	commitError             error
}

func (f *referenceFixture) Within(ctx context.Context, work func(context.Context, masterPorts.ReferenceRepository) error) error {
	f.units++
	e := work(ctx, f)
	if e != nil {
		return e
	}
	return f.commitError
}
func (f *referenceFixture) Resolve(context.Context, string) (sharedIdentity.Principal, error) {
	return sharedIdentity.Principal{ID: "actor", Active: true, Permissions: []string{f.permission}}, nil
}
func (f *referenceFixture) ID() string { return "new" }
func (f *referenceFixture) Get(_ context.Context, k, id string) (*masterDomain.Reference, error) {
	v, ok := f.records[k+id]
	if !ok {
		return nil, nil
	}
	return &v, nil
}
func (f *referenceFixture) List(context.Context, string, string, bool) ([]masterDomain.Reference, error) {
	return []masterDomain.Reference{}, nil
}
func (f *referenceFixture) LockMutations(context.Context) error { f.locked = true; return nil }
func (f *referenceFixture) Save(_ context.Context, k string, v masterDomain.Reference, _ bool) error {
	if !f.locked {
		panic("write before lock")
	}
	f.writes++
	f.records[k+v.ID] = v
	return nil
}
func (f *referenceFixture) Status(_ context.Context, k, id, actor string, active bool) error {
	if !f.locked {
		panic("status before lock")
	}
	f.writes++
	v := f.records[k+id]
	v.Active = active
	v.UpdatedBy = &actor
	f.records[k+id] = v
	return nil
}
func (f *referenceFixture) RetireCities(_ context.Context, id, actor string) error {
	if !f.locked {
		panic("cascade before lock")
	}
	f.cascades++
	for k, v := range f.records {
		if v.StateID == id && v.Active {
			v.Active = false
			v.UpdatedBy = &actor
			f.records[k] = v
		}
	}
	return nil
}
func TestReferenceCascadeAndGuardedActivation(t *testing.T) {
	f := &referenceFixture{permission: "settings.locations.write", records: map[string]masterDomain.Reference{"statesstate": {ID: "state", Active: true}, "citiescity": {ID: "city", StateID: "state", Active: true}}}
	s := NewReferenceAdministration(f, f, f)
	ctx := context.Background()
	if _, e := s.SetActive(ctx, "token", "states", "state", false); e != nil || f.records["citiescity"].Active || f.cascades != 1 {
		t.Fatal("cascade", e)
	}
	before := f.writes
	if _, e := s.SetActive(ctx, "token", "cities", "city", true); e == nil || f.writes != before {
		t.Fatal("inactive parent accepted")
	}
	if _, e := s.SetActive(ctx, "token", "states", "state", true); e != nil || f.records["citiescity"].Active {
		t.Fatal("state revived city")
	}
	if _, e := s.SetActive(ctx, "token", "cities", "city", true); e != nil || !f.records["citiescity"].Active {
		t.Fatal(e)
	}
	if v, e := s.SetActive(ctx, "token", "cities", "missing", true); v != nil || e != masterDomain.ReferenceMissing {
		t.Fatal("missing", e)
	}
}
func TestReferencePermissionAndCommitFailure(t *testing.T) {
	f := &referenceFixture{permission: "settings.locations.read", records: map[string]masterDomain.Reference{}}
	s := NewReferenceAdministration(f, f, f)
	if _, e := s.Save(context.Background(), "token", "states", "", masterDomain.ReferenceInput{Name: "State"}); e != masterDomain.ReferenceForbidden || f.units != 0 {
		t.Fatal("denied entered unit")
	}
	f.permission = "settings.locations.write"
	f.commitError = errors.New("commit failed")
	if v, e := s.Save(context.Background(), "token", "states", "", masterDomain.ReferenceInput{Name: "State"}); v != nil || e == nil {
		t.Fatal("uncommitted result returned")
	}
}
func TestReferenceCityCreateUpdateRequiresActiveParent(t *testing.T) {
	for _, parent := range []*masterDomain.Reference{nil, {ID: "state", Active: false}} {
		f := &referenceFixture{permission: "settings.locations.write", records: map[string]masterDomain.Reference{}}
		if parent != nil {
			f.records["statesstate"] = *parent
		}
		s := NewReferenceAdministration(f, f, f)
		if _, e := s.Save(context.Background(), "token", "cities", "", masterDomain.ReferenceInput{Name: "City", StateID: "state"}); e == nil || f.writes != 0 {
			t.Fatal("missing/inactive parent accepted")
		}
	}
}
