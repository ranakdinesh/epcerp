package application

import (
	"context"
	"errors"
	"testing"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type testRepository interface {
	ports.SessionRepository
	ports.UserRepository
}

type fakeUnit[R any] struct {
	repo               R
	commits, rollbacks int
}

func (u *fakeUnit[R]) Within(ctx context.Context, work func(context.Context, R) error) error {
	err := work(ctx, u.repo)
	if err == nil {
		u.commits++
	} else {
		u.rollbacks++
	}
	return err
}

type fakeTokens struct {
	ports.Tokens
	subject string
	now     int64
}

func (t fakeTokens) Subject(string) (string, error)                  { return t.subject, nil }
func (t fakeTokens) Now() int64                                      { return t.now }
func (t fakeTokens) ID() string                                      { return "fixture-id" }
func (t fakeTokens) HashRefresh(raw string) string                   { return "hash:" + raw }
func (t fakeTokens) Refresh() (string, string)                       { return "replacement", "replacement-hash" }
func (t fakeTokens) Issue(string, int64, int64) (string, int, error) { return "access", 60, nil }

type fakeRepo struct {
	testRepository
	user             *domain.User
	refresh          *domain.Refresh
	revoked, created int
}

func (r *fakeRepo) User(context.Context, string, bool) (*domain.User, error) { return r.user, nil }
func (r *fakeRepo) Refresh(context.Context, string, bool) (*domain.Refresh, error) {
	return r.refresh, nil
}
func (r *fakeRepo) Lock(context.Context, string) error          { return nil }
func (r *fakeRepo) RevokeFamily(context.Context, string) error  { r.revoked++; return nil }
func (r *fakeRepo) RevokeRefresh(context.Context, string) error { r.revoked++; return nil }
func (r *fakeRepo) CreateRefresh(_ context.Context, record domain.Refresh) error {
	r.created++
	if record.ExpiresAt.Unix() != 200 {
		return errors.New("absolute session expiry changed")
	}
	return nil
}
func (r *fakeRepo) ActiveSuperusers(context.Context) (int64, error) { return 1, nil }
func TestReplayDenialCommitsFamilyRevocation(t *testing.T) {
	revoked := domain.At(90)
	repo := &fakeRepo{user: &domain.User{IsActive: true}, refresh: &domain.Refresh{FamilyID: "family", ExpiresAt: domain.At(200), RevokedAt: &revoked}}
	unit := &fakeUnit[ports.SessionRepository]{repo: repo}
	service := NewSessions(unit, nil, fakeTokens{now: 100}, domain.Policy{})
	if _, err := service.Refresh(context.Background(), "old"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("replay authorized")
	}
	if repo.revoked != 1 || unit.commits != 1 || unit.rollbacks != 0 || repo.created != 0 {
		t.Fatal("denial rolled back security revocation")
	}
	repo.refresh.RevokedAt = nil
	if pair, err := service.Refresh(context.Background(), "current"); err != nil || pair.RefreshExpiresIn != 100 || repo.created != 1 {
		t.Fatal("rotation altered absolute expiry")
	}
}
func TestProtectedUseCasesRejectInactiveAndUnprivilegedActors(t *testing.T) {
	repo := &fakeRepo{user: &domain.User{ID: "actor", IsActive: true}}
	unit := &fakeUnit[ports.UserRepository]{repo: repo}
	service := NewUsers(unit, nil, fakeTokens{subject: "actor"}, domain.Policy{})
	if _, err := service.Users(context.Background(), "token", domain.UserFilter{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("unprivileged use case reached repository")
	}
	repo.user.IsActive = false
	if _, err := NewSessions(&fakeUnit[ports.SessionRepository]{repo: repo}, nil, fakeTokens{subject: "actor"}, domain.Policy{}).Me(context.Background(), "token"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("deactivated identity resolved")
	}
	repo.user.IsActive = true
	repo.user.IsSuperuser = true
	if _, err := service.Deactivate(context.Background(), "token", "actor"); !errors.Is(err, domain.ErrSelfDeactivation) {
		t.Fatal("self deactivation allowed")
	}
	if _, err := service.Deactivate(context.Background(), "token", "different"); !errors.Is(err, domain.ErrLastSuperadmin) {
		t.Fatal("last recovery identity allowed")
	}
}

func (t fakeTokens) Version(string) (int64, error) { return 0, nil }
