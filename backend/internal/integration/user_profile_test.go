package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/ranakdinesh/epcERP/helpers"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	profileOut "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	security "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	profileApp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	eventOut "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventApp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"image"
	"image/png"
	"testing"
	"time"
)

func TestProfileTransactionAndOwnScope(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	values, err := security.NewTokens(security.TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "fixture", Audience: "fixture", AccessLifetime: 15 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	first, second := values.ID(), values.ID()
	for i, id := range []string{first, second} {
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,'fixture',true,false)", id, []string{"profile_a", "profile_b"}[i], []string{"profile_a@example.invalid", "profile_b@example.invalid"}[i])
	}
	identities := projectIdentities{"first": {ID: first, Active: true, Permissions: []string{"account.profile.read", "account.profile.write"}}, "second": {ID: second, Active: true, Permissions: []string{"account.profile.read"}}, "inactive": {ID: first, Active: false, Superuser: true}}
	unit := profileOut.NewProfileUnitOfWork(pool, func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) })
	var encryptionKey [32]byte
	_, _ = rand.Read(encryptionKey[:])
	cipher, err := infrastructure.NewDocumentCipher(hex.EncodeToString(encryptionKey[:]))
	if err != nil {
		t.Fatal(err)
	}
	objectStore := &objects{data: map[string][]byte{}}
	images, err := infrastructure.NewImageStorage(objectStore, cipher, "fixture-images", domain.ProfileMaximumPhotoBytes)
	if err != nil {
		t.Fatal(err)
	}
	service := profileApp.NewProfile(unit, infrastructure.NewAuthBridge(identities), helpers.ImageProcessor{MaximumBytes: domain.ProfileMaximumPhotoBytes, MaximumDimension: 2048, Size: 256}, values, images)
	name := "First Staff"
	p, e := service.Update(ctx, "first", &name, nil, false)
	if e != nil || p.UserID != first || p.Version != 1 {
		t.Fatal("own update failed", e)
	}
	if _, e = service.Update(ctx, "second", &name, nil, false); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("read-only identity wrote", e)
	}
	if _, e = service.Update(ctx, "inactive", &name, nil, false); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("inactive write", e)
	}
	if _, e = service.Get(ctx, "", first); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("anonymous read", e)
	}
	var raw bytes.Buffer
	_ = png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	p, e = service.Update(ctx, "first", nil, raw.Bytes(), true)
	if e != nil || p.PhotoObjectKey == "" || len(p.Photo) != 0 || p.DisplayName != name || p.Version != 2 {
		t.Fatal("photo/name preservation", e)
	}
	other, e := service.Get(ctx, "second", first)
	if e != nil || other.DisplayName != name || other.PhotoObjectKey == "" {
		t.Fatal("staff presentation unavailable", e)
	}
	canonical, e := service.Image(ctx, "second", first)
	if e != nil || len(canonical) == 0 {
		t.Fatal("encrypted image unavailable", e)
	}
	key := p.PhotoObjectKey
	encrypted, e := objectStore.Get(ctx, "fixture-images", key)
	if e != nil || bytes.Equal(encrypted, canonical) {
		t.Fatal("object stored in plaintext", e)
	}
	objectStore.data["fixture-images/"+key][0] ^= 1
	if _, e = service.Image(ctx, "second", first); e == nil {
		t.Fatal("tampered image accepted")
	}
	objectStore.data["fixture-images/"+key][0] ^= 1
	objectStore.reject = true
	if _, e = service.Update(ctx, "first", nil, raw.Bytes(), true); e == nil {
		t.Fatal("failed image store accepted")
	}
	objectStore.reject = false
	current, e := service.Get(ctx, "first", first)
	if e != nil || current.Version != 2 || current.PhotoObjectKey != key {
		t.Fatal("storage failure changed metadata", e)
	}
	p, e = service.Update(ctx, "first", nil, nil, true)
	if e != nil || p.PhotoObjectKey != "" || len(p.Photo) != 0 || p.Version != 3 {
		t.Fatal("removal", e)
	}
	if image, e := service.Image(ctx, "first", first); e != nil || len(image) != 0 {
		t.Fatal("removed image still exposed", e)
	}
	// Existing database photos remain readable without new object writes.
	execFixture(t, pool, "INSERT INTO auth.user_profile(user_id,photo,created_by_id,updated_by_id) VALUES($1,$2,$1,$1)", second, canonical)
	if image, e := service.Image(ctx, "first", second); e != nil || !bytes.Equal(image, canonical) {
		t.Fatal("legacy photo unavailable", e)
	}
	var actor string
	var n int
	if e = pool.QueryRow(ctx, "SELECT updated_by_id::text FROM auth.user_profile WHERE user_id=$1", first).Scan(&actor); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM business_events WHERE source_module='auth' AND entity_type='user_profile' AND entity_id=$1 AND actor_id=$1", first).Scan(&n); e != nil {
		t.Fatal(e)
	}
	if actor != first || n != 3 {
		t.Fatal("audit attribution missing")
	}
	execFixture(t, pool, `CREATE FUNCTION reject_profile_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END; $$; CREATE CONSTRAINT TRIGGER reject_profile_event AFTER INSERT ON business_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_profile_event();`)
	name = "Rolled Back"
	if _, e = service.Update(ctx, "first", &name, raw.Bytes(), true); e == nil {
		t.Fatal("event commit failure accepted")
	}
	p, e = service.Get(ctx, "first", first)
	if e != nil || p.Version != 3 || p.DisplayName == name {
		t.Fatal("mutation not rolled back", e)
	}
}

func TestPersonalContactSelfEditAndAudit(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	values, e := security.NewTokens(security.TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "fixture", Audience: "fixture", AccessLifetime: 15 * time.Minute})
	if e != nil {
		t.Fatal(e)
	}
	first, second, department := values.ID(), values.ID(), values.ID()
	for i, id := range []string{first, second} {
		execFixture(t, pool, "INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,'fixture',true,false)", id, []string{"contact_a", "contact_b"}[i], []string{"a@example.invalid", "b@example.invalid"}[i])
	}
	ids := projectIdentities{"first": {ID: first, Active: true, Permissions: []string{"account.profile.read", "account.profile.write"}}, "second": {ID: second, Active: true, Permissions: []string{"account.profile.read"}}, "inactive": {ID: first, Active: false}}
	unit := profileOut.NewProfileUnitOfWork(pool, func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) })
	s := profileApp.NewProfile(unit, infrastructure.NewAuthBridge(ids), nil, values, nil)
	request := domain.PersonalDetails{Name: "Contact A", Contact: domain.PersonalContact{FirstName: "First", LastName: "Employee", Mobile: "+91 98765 43210"}}
	p, e := s.UpdateDetails(ctx, "first", request)
	if e != nil || p.Contact == nil || p.Contact.FirstName != "First" || p.Version != 1 {
		t.Fatal("legacy employee could not self-complete", e)
	}
	if _, e = s.UpdateDetails(ctx, "first", request); !errors.Is(e, domain.ErrProfileConflict) {
		t.Fatal("stale form accepted", e)
	}
	if _, e = s.UpdateDetails(ctx, "second", request); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("read-only caller wrote", e)
	}
	if _, e = s.UpdateDetails(ctx, "inactive", request); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("inactive caller wrote", e)
	}
	other, e := s.Get(ctx, "second", first)
	if e != nil || other.Contact != nil {
		t.Fatal("private contact leaked through staff presentation", e)
	}
	var name, actor string
	var n int
	if e = pool.QueryRow(ctx, "SELECT first_name,updated_by_id::text FROM auth.users WHERE id=$1", first).Scan(&name, &actor); e != nil || name != "First" || actor != first {
		t.Fatal("canonical contact/actor missing", e)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM auth.auth_personal_contact_changes WHERE user_id=$1 AND actor_id=$1 AND before_snapshot->>'first_name'='' AND after_snapshot->>'first_name'='First'", first).Scan(&n); e != nil || n != 1 {
		t.Fatal("contact snapshots missing", e)
	}
	if _, e = pool.Exec(ctx, "UPDATE auth.auth_personal_contact_changes SET reason='Tampering with history' WHERE user_id=$1", first); e == nil {
		t.Fatal("history mutable")
	}
	execFixture(t, pool, "INSERT INTO auth.auth_departments(id,code,name) VALUES($1,'FIXTURE','Fixture department')", department)
	execFixture(t, pool, "INSERT INTO auth.auth_reporting_lines(user_id,department_id,is_company_root) VALUES($1,$2,true)", first, department)
	request.Expected = request.Contact
	request.Contact.FirstName = "Updated"
	request.Version = 1
	p, e = s.UpdateDetails(ctx, "first", request)
	if e != nil || p.Version != 2 {
		t.Fatal("configured employee update", e)
	}
	if e = pool.QueryRow(ctx, "SELECT version FROM auth.auth_reporting_lines WHERE user_id=$1", first).Scan(&n); e != nil || n != 2 {
		t.Fatal("open admin form not invalidated", e)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM auth.auth_organisation_changes WHERE entity_id=$1 AND version=2", first).Scan(&n); e != nil || n != 1 {
		t.Fatal("organisation snapshot missing", e)
	}
	request.Expected = request.Contact
	request.Version = 2
	execFixture(t, pool, "UPDATE auth.users SET mobile='+91 91234 56789' WHERE id=$1", first)
	if _, e = s.UpdateDetails(ctx, "first", request); !errors.Is(e, domain.ErrProfileConflict) {
		t.Fatal("admin contact correction overwritten", e)
	}
	request.Expected.Mobile = "+91 91234 56789"
	request.Contact.FirstName = "Rollback"
	execFixture(t, pool, `CREATE FUNCTION reject_contact_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END; $$; CREATE CONSTRAINT TRIGGER reject_contact_event AFTER INSERT ON business_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_contact_event();`)
	if _, e = s.UpdateDetails(ctx, "first", request); e == nil {
		t.Fatal("audit failure accepted")
	}
	p, e = s.Get(ctx, "first", "")
	if e != nil || p.Version != 2 || p.Contact.FirstName != "Updated" {
		t.Fatal("contact transaction did not roll back", e)
	}
	if e = pool.QueryRow(ctx, "SELECT version FROM auth.auth_reporting_lines WHERE user_id=$1", first).Scan(&n); e != nil || n != 2 {
		t.Fatal("organisation version did not roll back", e)
	}
}
