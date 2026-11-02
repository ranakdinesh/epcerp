package integration

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	out "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	app "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func TestUserAdminEditPersistsCanonicalContact(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	var migration int
	var dirty bool
	if err := pool.QueryRow(ctx, `SELECT version, dirty FROM public.schema_migrations`).Scan(&migration, &dirty); err != nil || dirty {
		t.Fatalf("migration state: version=%d dirty=%v error=%v", migration, dirty, err)
	}
	t.Logf("tested migration version=%d dirty=%v", migration, dirty)
	tokens, err := security.NewTokens(security.TokenConfig{Secret: rand.Text() + rand.Text(), Issuer: "fixture", Audience: "fixture", AccessLifetime: 15 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	admin, staff := tokens.ID(), tokens.ID()
	for i, id := range []string{admin, staff} {
		execFixture(t, pool, `INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser,first_name,last_name,mobile) VALUES($1,$2,$3,'fixture',true,$4,'Original','Name','9000000000')`, id, []string{"edit_admin", "edit_staff"}[i], []string{"admin@example.invalid", "staff@example.invalid"}[i], i == 0)
	}
	token, _, err := tokens.Issue(admin, tokens.Now()+3600, 0)
	if err != nil {
		t.Fatal(err)
	}
	mail := &fixtureEmails{enabled: true}
	service := app.NewUsers(out.NewUserUnitOfWork(pool), nil, tokens, domain.Policy{}).WithEmailSender(mail, "https://example.invalid")
	first, last, mobile := "Asha", "Shah", "+91 90000 00001"
	updated, err := service.UpdateUser(ctx, token, staff, domain.UserUpdate{FirstName: &first, LastName: &last, Mobile: &mobile})
	if err != nil {
		t.Fatal(err)
	}
	if updated.FirstName != first || updated.LastName != last || updated.Mobile != mobile || !updated.IsActive {
		t.Fatalf("unexpected update: %+v", updated)
	}
	email := "new@example.invalid"
	if _, err = service.UpdateUser(ctx, token, staff, domain.UserUpdate{Email: &email}); err != nil {
		t.Fatal(err)
	}
	saved, err := service.User(ctx, token, staff)
	if err != nil || saved.FirstName != first || saved.LastName != last || saved.Mobile != mobile || saved.Email != "staff@example.invalid" || len(mail.messages) != 1 {
		t.Fatalf("partial update lost contact: %+v %v", saved, err)
	}
	var actor string
	if err = pool.QueryRow(ctx, `SELECT updated_by_id::text FROM auth.users WHERE id=$1`, staff).Scan(&actor); err != nil || actor != admin {
		t.Fatalf("update actor: %s %v", actor, err)
	}
	staffToken, _, err := tokens.Issue(staff, tokens.Now()+3600, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.UpdateUser(ctx, staffToken, admin, domain.UserUpdate{FirstName: &first}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("unprivileged update: %v", err)
	}
	if inactive, err := service.Deactivate(ctx, token, staff); err != nil || inactive.IsActive {
		t.Fatalf("immediate access revocation failed: %v", err)
	}
}
