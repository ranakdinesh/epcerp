package application

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type SessionService struct {
	unit      ports.UnitOfWork[ports.SessionRepository]
	passwords ports.Passwords
	tokens    ports.Tokens
	policy    domain.Policy
}

func NewSessions(unit ports.UnitOfWork[ports.SessionRepository], passwords ports.Passwords, tokens ports.Tokens, policy domain.Policy) *SessionService {
	return &SessionService{unit, passwords, tokens, policy}
}

func (s *SessionService) Me(ctx context.Context, token string) (*domain.User, error) {
	var result *domain.User
	err := s.unit.Within(ctx, func(ctx context.Context, repo ports.SessionRepository) error {
		var err error
		result, err = resolveSelf(ctx, repo, s.tokens, token)
		return err
	})
	return result, err
}

func (s *SessionService) pair(user domain.User, raw string, expiry int64) (domain.TokenPair, error) {
	access, lifetime, err := s.tokens.Issue(user.ID, expiry, user.CredentialVersion)
	remaining := int(expiry - s.tokens.Now())
	if remaining < 0 {
		remaining = 0
	}
	return domain.TokenPair{MustChangePassword: user.MustChangePassword, AccessToken: access, RefreshToken: raw, TokenType: "bearer", ExpiresIn: lifetime, RefreshExpiresIn: remaining}, err
}

func (s *SessionService) Login(ctx context.Context, identifier, password string) (domain.TokenPair, error) {
	var pair domain.TokenPair
	err := s.unit.Within(ctx, func(ctx context.Context, repo ports.SessionRepository) error {
		user, err := repo.FindUser(ctx, identifier)
		if err != nil {
			return err
		}
		if user == nil {
			s.passwords.VerifyAbsent(ctx, password)
			return domain.ErrUnauthorized
		}
		if !s.passwords.Verify(ctx, password, user.PasswordHash) || !user.IsActive || !user.TemporaryValid(s.tokens.Now()) {
			return domain.ErrUnauthorized
		}
		locked, err := repo.User(ctx, user.ID, true)
		if err != nil {
			return err
		}
		if locked == nil || !locked.IsActive || locked.PasswordHash != user.PasswordHash {
			return domain.ErrUnauthorized
		}
		raw, hash := s.tokens.Refresh()
		expiry := s.tokens.Now() + int64(s.policy.SessionLifetime.Seconds())
		if locked.MustChangePassword && expiry > s.tokens.Now()+900 {
			expiry = s.tokens.Now() + 900
		}
		if err = repo.CreateRefresh(ctx, domain.Refresh{ID: s.tokens.ID(), UserID: user.ID, Hash: hash, FamilyID: s.tokens.ID(), ExpiresAt: domain.At(expiry)}); err != nil {
			return err
		}
		pair, err = s.pair(*locked, raw, expiry)
		return err
	})
	return pair, err
}

func (s *SessionService) Refresh(ctx context.Context, raw string) (domain.TokenPair, error) {
	var pair domain.TokenPair
	valid := false
	err := s.unit.Within(ctx, func(ctx context.Context, repo ports.SessionRepository) error {
		hash := s.tokens.HashRefresh(raw)
		initial, err := repo.Refresh(ctx, hash, false)
		if err != nil || initial == nil {
			return err
		}
		user, err := repo.User(ctx, initial.UserID, true)
		if err != nil {
			return err
		}
		if err = repo.Lock(ctx, "auth.refresh.family:"+initial.FamilyID); err != nil {
			return err
		}
		token, err := repo.Refresh(ctx, hash, true)
		if err != nil {
			return err
		}
		if token == nil || token.ExpiresAt.Unix() <= s.tokens.Now() || user == nil || !user.IsActive || !user.TemporaryValid(s.tokens.Now()) {
			return nil
		}
		if token.RevokedAt != nil {
			return repo.RevokeFamily(ctx, token.FamilyID)
		}
		replacement, replacementHash := s.tokens.Refresh()
		if err = repo.RevokeRefresh(ctx, hash); err != nil {
			return err
		}
		if err = repo.CreateRefresh(ctx, domain.Refresh{ID: s.tokens.ID(), UserID: user.ID, Hash: replacementHash, FamilyID: token.FamilyID, ExpiresAt: token.ExpiresAt}); err != nil {
			return err
		}
		pair, err = s.pair(*user, replacement, token.ExpiresAt.Unix())
		valid = err == nil
		return err
	})
	if err != nil {
		return domain.TokenPair{}, err
	}
	if !valid {
		return domain.TokenPair{}, domain.ErrUnauthorized
	}
	return pair, nil
}

func (s *SessionService) Logout(ctx context.Context, raw string) error {
	return s.unit.Within(ctx, func(ctx context.Context, repo ports.SessionRepository) error {
		hash := s.tokens.HashRefresh(raw)
		initial, err := repo.Refresh(ctx, hash, false)
		if err != nil || initial == nil {
			return err
		}
		if _, err = repo.User(ctx, initial.UserID, true); err != nil {
			return err
		}
		if err = repo.Lock(ctx, "auth.refresh.family:"+initial.FamilyID); err != nil {
			return err
		}
		return repo.RevokeRefresh(ctx, hash)
	})
}

func (s *SessionService) Activity(ctx context.Context, token, page string) error {
	return s.unit.Within(ctx, func(ctx context.Context, repo ports.SessionRepository) error {
		user, err := resolve(ctx, repo, s.tokens, token)
		if err != nil {
			return err
		}
		return repo.RecordActivity(ctx, user.ID, page)
	})
}
