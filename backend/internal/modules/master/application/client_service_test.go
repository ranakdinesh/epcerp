package application

import (
	"context"
	"errors"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
	"testing"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type clientFixture struct {
	record  *masterDomain.Client
	retired bool
	calls   int
	fail    error
}

func (f *clientFixture) Resolve(context.Context, string) (sharedIdentity.Principal, error) {
	return sharedIdentity.Principal{ID: "actor", Active: true, Permissions: []string{"settings.clients.read", "settings.clients.write"}}, nil
}
func (f *clientFixture) ID() string { return "00112233-4455-4677-8899-aabbccddeeff" }
func (f *clientFixture) Get(_ context.Context, id string, _ bool) (*masterDomain.Client, error) {
	return f.record, nil
}

// A separate directory fixture is needed because the consumer ports intentionally differ.
type clientDirectory struct{}

func (clientDirectory) Get(_ context.Context, k, id string) (*masterDomain.ClientReference, error) {
	name := k
	if k == "states" {
		name = "Maharashtra"
	}
	return &masterDomain.ClientReference{ID: id, Name: name, Active: true, StateID: "state"}, nil
}

type clientUnit struct{ f *clientFixture }

func (u clientUnit) Within(ctx context.Context, work func(context.Context, masterPorts.ClientRepository, masterPorts.ClientDirectory) error) error {
	u.f.calls++
	return work(ctx, u.f, clientDirectory{})
}
func (f *clientFixture) List(context.Context, masterDomain.ClientFilter) (masterDomain.ClientPage, error) {
	return masterDomain.ClientPage{}, nil
}
func (f *clientFixture) Save(_ context.Context, c masterDomain.Client, _ bool) error {
	f.record = &c
	return f.fail
}
func (f *clientFixture) Contacts(_ context.Context, _ string, c []masterDomain.ClientContact, _ string, retire bool) error {
	f.retired = retire
	if !retire {
		f.record.Contacts = c
	}
	return f.fail
}
func TestClientLifecycleAndForeignContacts(t *testing.T) {
	f := &clientFixture{record: &masterDomain.Client{ID: "client", Active: true, Contacts: []masterDomain.ClientContact{{ID: "historic", Active: false}}}}
	s := NewClientService(clientUnit{f}, f, f)
	ctx := context.Background()
	if _, e := s.SetActive(ctx, "token", "client", true); e != nil || f.retired {
		t.Fatal("activation revived contacts")
	}
	if _, e := s.SetActive(ctx, "token", "client", false); e != nil || !f.retired {
		t.Fatal("deactivation")
	}
	in := masterDomain.ClientInput{StateID: "state", Gstin: "27AAACW4202F1ZQ", Contacts: []masterDomain.ClientContact{{ID: "foreign"}}}
	if _, e := s.Save(ctx, "token", "client", in); e == nil {
		t.Fatal("foreign contact accepted")
	}
	f.fail = errors.New("commit failure")
	if c, e := s.SetActive(ctx, "token", "client", true); e == nil || c != nil {
		t.Fatal("failed write returned record")
	}
}

type clientDenied struct{}

func (clientDenied) Resolve(context.Context, string) (sharedIdentity.Principal, error) {
	return sharedIdentity.Principal{ID: "reader", Active: true, Permissions: []string{"settings.clients.read"}}, nil
}
func TestClientDeniedWriteNeverEntersUnit(t *testing.T) {
	f := &clientFixture{}
	s := NewClientService(clientUnit{f}, clientDenied{}, f)
	if _, e := s.Save(context.Background(), "token", "", masterDomain.ClientInput{}); e != masterDomain.ClientForbidden || f.calls != 0 {
		t.Fatal("unauthorized mutation")
	}
}
