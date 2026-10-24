package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/ports"
	"testing"
)

type identity struct{ principal domain.Principal }

func (i identity) Resolve(context.Context, string) (domain.Principal, error) { return i.principal, nil }

type repository struct {
	ports.Repository
	doc       *domain.Document
	events    []domain.Event
	failEvent bool
	locked    bool
}

func (r *repository) Save(_ context.Context, doc domain.Document) (domain.Document, error) {
	r.doc = &doc
	return doc, nil
}
func (r *repository) Get(_ context.Context, _ string, lock bool) (*domain.Document, error) {
	r.locked = lock
	return r.doc, nil
}
func (r *repository) List(context.Context, string, string) ([]domain.Document, error) {
	if r.doc == nil || !r.doc.Active {
		return []domain.Document{}, nil
	}
	return []domain.Document{*r.doc}, nil
}
func (r *repository) Deactivate(_ context.Context, _ string, actor string) (*domain.Document, error) {
	r.doc.Active = false
	r.doc.UpdatedBy = &actor
	return r.doc, nil
}
func (r *repository) Event(_ context.Context, event domain.Event) error {
	if r.failEvent {
		return errors.New("fixture audit failure")
	}
	r.events = append(r.events, event)
	return nil
}

type transaction struct {
	repo               *repository
	commits, rollbacks int
}

func (u *transaction) Within(ctx context.Context, work func(context.Context, ports.Repository) error) error {
	err := work(ctx, u.repo)
	if err != nil {
		u.rollbacks++
	} else {
		u.commits++
	}
	return err
}

type objects struct {
	data       []byte
	puts, gets int
	failPut    bool
}

func (o *objects) Put(_ context.Context, _, _ string, data []byte) error {
	o.puts++
	if o.failPut {
		return errors.New("fixture storage failure")
	}
	o.data = data
	return nil
}
func (o *objects) Get(context.Context, string, string) ([]byte, error) { o.gets++; return o.data, nil }

type cryptography struct{}

func (cryptography) Encrypt(data []byte) ([]byte, error) {
	return append([]byte("encrypted:"), data...), nil
}
func (cryptography) Decrypt(data []byte) ([]byte, error) {
	if len(data) < 10 {
		return nil, errors.New("fixture corrupt")
	}
	return data[10:], nil
}
func (cryptography) Hash(data []byte) string { return "hash:" + string(data) }
func (cryptography) Algorithm() string       { return "AES-256-GCM" }

type values struct{}

func (values) ID() string { return "12345678-1234-4234-8234-123456789abc" }
func (values) Now() int64 { return 1700000000000000000 }
func fixture() (*Service, *transaction, *objects) {
	repo := &repository{}
	unit := &transaction{repo: repo}
	store := &objects{}
	return New(unit, identity{domain.Principal{ID: "actor", Active: true, Permissions: []string{"documents.storage.read", "documents.storage.write"}}}, store, cryptography{}, values{}, "private-bucket"), unit, store
}
func TestLifecycleLocksAndAuditBeforePayloadRelease(t *testing.T) {
	service, unit, objects := fixture()
	request := domain.Upload{EntityType: " Supplier ", EntityID: values{}.ID(), DocType: "PAN", Title: "Private fixture", Filename: "../../../secret.pdf", ContentType: "application/pdf", Data: []byte("fixture")}
	doc, err := service.Upload(context.Background(), "token", request)
	if err != nil {
		t.Fatal(err)
	}
	if doc.EntityType != "supplier" || doc.Key != " Supplier /"+request.EntityID+"/"+doc.ID+"_secret.pdf.enc" || objects.puts != 1 || len(unit.repo.events) != 1 || unit.repo.events[0].Snapshot["entity_type"] != request.EntityType {
		t.Fatal("upload metadata/snapshot drift")
	}
	_, data, err := service.Download(context.Background(), "token", doc.ID)
	if err != nil || string(data) != "fixture" || !unit.repo.locked || unit.repo.events[1].Action != "DOWNLOAD" {
		t.Fatal("download/audit contract failed")
	}
	unit.repo.failEvent = true
	_, data, err = service.Download(context.Background(), "token", doc.ID)
	if err == nil || data != nil {
		t.Fatal("payload released after failed audit")
	}
	unit.repo.failEvent = false
	objects.data = []byte("encrypted:corrupted")
	_, data, err = service.Download(context.Background(), "token", doc.ID)
	if !errors.Is(err, domain.ErrIntegrity) || data != nil {
		t.Fatal("tampered hash released")
	}
	if err = service.Deactivate(context.Background(), "token", doc.ID); err != nil || unit.repo.doc.Active || unit.repo.events[len(unit.repo.events)-1].Action != "DEACTIVATE" {
		t.Fatal("deactivation failed")
	}
	_, data, err = service.Download(context.Background(), "token", doc.ID)
	if err == nil || data != nil {
		t.Fatal("inactive data released")
	}
}
func TestPermissionAndAuditFailuresDoNotTouchObjects(t *testing.T) {
	service, unit, objects := fixture()
	service.auth = identity{domain.Principal{ID: "actor", Active: true}}
	_, err := service.Upload(context.Background(), "token", domain.Upload{})
	var denied *domain.Error
	if !errors.As(err, &denied) || denied.Code != 403 || objects.puts != 0 || unit.commits != 0 {
		t.Fatal("unauthorized use case touched storage")
	}
	service.auth = identity{domain.Principal{ID: "actor", Active: true, Superuser: true}}
	unit.repo.failEvent = true
	request := domain.Upload{EntityType: "supplier", EntityID: values{}.ID(), DocType: "pan", ContentType: "application/pdf", Data: []byte("fixture")}
	if _, err = service.Upload(context.Background(), "token", request); err == nil || objects.puts != 0 || unit.rollbacks != 1 {
		t.Fatal("audit failure touched objects or committed")
	}
}
