package application

import (
	"context"
	"errors"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
	"testing"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type manufacturerFixture struct {
	record                  masterDomain.Manufacturer
	events                  []masterDomain.ManufacturerEvent
	locked, rowLocked       bool
	units                   int
	permission              string
	eventError, commitError error
}

func (f *manufacturerFixture) Within(ctx context.Context, work func(context.Context, masterPorts.ManufacturerRepository, masterPorts.ManufacturerEligibility) error) error {
	f.units++
	if e := work(ctx, f, f); e != nil {
		return e
	}
	return f.commitError
}
func (f *manufacturerFixture) Resolve(context.Context, string) (sharedIdentity.Principal, error) {
	return sharedIdentity.Principal{ID: "actor", Active: true, Permissions: []string{f.permission}}, nil
}
func (f *manufacturerFixture) ID() string { return "new" }
func (f *manufacturerFixture) Validate(_ context.Context, d masterDomain.ManufacturerData) error {
	return masterDomain.ManufacturerValidateEligibility(d, []string{"ACTIVE"})
}
func (f *manufacturerFixture) Lock(context.Context) error { f.locked = true; return nil }
func (f *manufacturerFixture) List(context.Context) ([]masterDomain.Manufacturer, error) {
	if f.record.ID == "" {
		return nil, nil
	}
	return []masterDomain.Manufacturer{f.record}, nil
}
func (f *manufacturerFixture) Get(_ context.Context, id string, lock bool) (*masterDomain.Manufacturer, error) {
	f.rowLocked = lock
	if id != f.record.ID {
		return nil, nil
	}
	v := f.record
	return &v, nil
}
func (f *manufacturerFixture) Create(_ context.Context, v masterDomain.Manufacturer) (masterDomain.Manufacturer, error) {
	if !f.locked {
		panic("unlocked create")
	}
	f.record = v
	return v, nil
}
func (f *manufacturerFixture) Update(_ context.Context, v masterDomain.Manufacturer, _ bool) (masterDomain.Manufacturer, error) {
	if !f.rowLocked {
		panic("unlocked version update")
	}
	f.record = v
	return v, nil
}
func (f *manufacturerFixture) Event(_ context.Context, v masterDomain.ManufacturerEvent) error {
	f.events = append(f.events, v)
	return f.eventError
}
func (f *manufacturerFixture) History(context.Context, string) ([]masterDomain.ManufacturerEvent, error) {
	return f.events, nil
}
func TestManufacturerVersionedAuditSnapshots(t *testing.T) {
	f := &manufacturerFixture{permission: "settings.manufacturers.write"}
	s := NewManufacturerService(f, f, f)
	ctx := context.Background()
	v, e := s.Create(ctx, "token", "MAKE", "Manufacturer", masterDomain.ManufacturerData{Eligible: []string{"ACTIVE"}}, "create reason")
	if e != nil || v.Version != 1 || len(f.events) != 1 || len(f.events[0].Before) != 0 || f.events[0].After["code"] != "MAKE" {
		t.Fatal(v, e)
	}
	v, e = s.Profile(ctx, "token", v.ID, 1, "Renamed", masterDomain.ManufacturerData{Eligible: []string{"ACTIVE"}, Notes: "changed"}, "edit reason")
	if e != nil || v.Version != 2 || f.events[1].Before["name"] != "Manufacturer" || f.events[1].After["version"] != int32(2) {
		t.Fatal(v, e)
	}
	if _, e = s.SetActive(ctx, "token", v.ID, 1, false, "stale"); e != masterDomain.ManufacturerStale || len(f.events) != 2 {
		t.Fatal("stale write emitted audit")
	}
	v, e = s.SetActive(ctx, "token", v.ID, 2, false, "lifecycle reason")
	if e != nil || v.Version != 3 || v.Active || f.events[2].Action != "lifecycle" {
		t.Fatal(v, e)
	}
	v, e = s.SetActive(ctx, "token", v.ID, 3, false, "repeat")
	if e != nil || v.Version != 4 {
		t.Fatal("repeat lifecycle did not advance")
	}
}
func TestManufacturerDeniedAndUncommittedResults(t *testing.T) {
	f := &manufacturerFixture{permission: "settings.manufacturers.read"}
	s := NewManufacturerService(f, f, f)
	if _, e := s.Create(context.Background(), "token", "MAKE", "Make", masterDomain.ManufacturerData{}, "reason"); e != masterDomain.ManufacturerForbidden || f.units != 0 {
		t.Fatal("denied entered unit")
	}
	f.permission = "settings.manufacturers.write"
	f.eventError = errors.New("audit failure")
	if v, e := s.Create(context.Background(), "token", "MAKE", "Make", masterDomain.ManufacturerData{Eligible: []string{"ACTIVE"}}, "reason"); v != nil || e == nil {
		t.Fatal("unaudited record returned")
	}
	f.eventError = nil
	f.record = masterDomain.Manufacturer{}
	f.commitError = errors.New("late failure")
	if v, e := s.Create(context.Background(), "token", "MAKE", "Make", masterDomain.ManufacturerData{Eligible: []string{"ACTIVE"}}, "reason"); v != nil || e == nil {
		t.Fatal("uncommitted record returned")
	}
}

func (f *manufacturerFixture) Options(context.Context, string, []string, int32, string, string) (masterDomain.ManufacturerSelection, error) {
	return masterDomain.ManufacturerSelection{}, nil
}
func (f *manufacturerFixture) FindImport(context.Context, string) (*masterDomain.Manufacturer, string, error) {
	return nil, "", nil
}
func (f *manufacturerFixture) RecordImport(context.Context, string, masterDomain.ManufacturerLegacy, string, string) error {
	return nil
}
func (f *manufacturerFixture) Fingerprint(masterDomain.ManufacturerLegacy) string { return "fixture" }
