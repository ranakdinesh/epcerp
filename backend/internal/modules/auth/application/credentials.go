package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type CredentialService struct {
	unit      ports.UnitOfWork[ports.CredentialRepository]
	passwords ports.Passwords
	values    ports.Tokens
	temporary ports.TemporaryPasswords
	mail      ports.EmailSender
	loginURL  string
}

func NewCredentials(u ports.UnitOfWork[ports.CredentialRepository], p ports.Passwords, v ports.Tokens, t ports.TemporaryPasswords, m ports.EmailSender, loginURL string) *CredentialService {
	return &CredentialService{u, p, v, t, m, loginURL}
}
func (s *CredentialService) issue(ctx context.Context, r ports.CredentialRepository, user domain.User, actor, reason string, reset bool) error {
	if !s.mail.Ready() {
		return &domain.Error{Kind: domain.Unavailable, Detail: "Email delivery is not configured. Configure the verified sender before creating or resetting users."}
	}
	if !user.IsActive {
		return &domain.Error{Kind: domain.Invalid, Detail: "Inactive users cannot receive a temporary password."}
	}
	raw, e := s.temporary.Generate()
	if e != nil {
		return e
	}
	if !domain.StrongPassword(raw) {
		return &domain.Error{Kind: domain.Invalid, Detail: "Temporary credential generation failed"}
	}
	hash, e := s.passwords.Hash(ctx, raw)
	if e != nil {
		return e
	}
	if e = r.SetTemporaryPassword(ctx, user.ID, hash, actor); e != nil {
		return e
	}
	current, e := r.User(ctx, user.ID, false)
	if e != nil {
		return e
	}
	if current == nil {
		return domain.ErrUserMissing
	}
	subject, message := domain.CredentialMessage(user.Username, raw, s.loginURL, reset)
	if e = r.QueueCredentialEmail(ctx, domain.CredentialEmail{ID: s.values.ID(), UserID: user.ID, Recipient: user.Email, Subject: subject, Message: message}, current.CredentialVersion, actor); e != nil {
		return e
	}
	if e = r.RevokeUserSessions(ctx, user.ID); e != nil {
		return e
	}
	action := "user_created"
	if user.IsSuperuser && !reset {
		reason = "New superadmin account; full platform access"
	}
	if reset {
		action = "temporary_password_reset"
	}
	return r.RecordCredentialChange(ctx, s.values.ID(), user.ID, actor, action, reason)
}
func (s *CredentialService) Create(ctx context.Context, token string, request domain.UserCreate) (*domain.User, error) {
	var result *domain.User
	if !domain.ValidEmployeeContact(request.FirstName, request.LastName, request.Mobile) {
		return nil, &domain.Error{Kind: domain.Invalid, Detail: "First name, last name and a valid mobile number are required."}
	}
	e := authorized(ctx, s.unit, s.values, token, "admin.users.write", true, func(ctx context.Context, r ports.CredentialRepository, actor domain.User) error {
		if request.IsSuperuser {
			if !actor.IsSuperuser {
				return domain.ErrForbidden
			}
			count, err := r.Superusers(ctx)
			if err != nil {
				return err
			}
			if count >= 2 {
				return domain.ErrSuperadminLimit
			}
		}
		request.RoleNames = domain.NormalizedRoles(request.RoleNames)
		if e := rolesValid(ctx, r, request.RoleNames, domain.ErrUserConflict); e != nil {
			return e
		}
		// A placeholder hash is never committed: issue and encrypted queue share this transaction.
		user, e := r.CreateUser(ctx, s.values.ID(), request, "pending-credential", actor.ID)
		if e != nil {
			return e
		}
		if e = s.issue(ctx, r, *user, actor.ID, "New employee account", false); e != nil {
			return e
		}
		result, e = r.User(ctx, user.ID, false)
		return e
	})
	return result, e
}
func (s *CredentialService) Reset(ctx context.Context, token, userID, reason string) error {
	if !domain.ValidResetReason(reason) {
		return &domain.Error{Kind: domain.Invalid, Detail: "Provide a reset reason of 10 to 1000 characters."}
	}
	return authorized(ctx, s.unit, s.values, token, "admin.users.write", true, func(ctx context.Context, r ports.CredentialRepository, actor domain.User) error {
		if userID == actor.ID {
			return &domain.Error{Kind: domain.Invalid, Detail: "Use your account security settings to change your own password."}
		}
		user, e := r.User(ctx, userID, true)
		if e != nil {
			return e
		}
		if user == nil {
			return domain.ErrUserMissing
		}
		if user.IsSuperuser && !actor.IsSuperuser {
			return domain.ErrForbidden
		}
		return s.issue(ctx, r, *user, actor.ID, reason, true)
	})
}

var ErrCredentialDelivery = errors.New("credential email delivery was not accepted")

type CredentialDeliveryService struct {
	unit   ports.UnitOfWork[ports.EmailDeliveryRepository]
	mail   ports.EmailSender
	values ports.Tokens
}

func NewCredentialDelivery(u ports.UnitOfWork[ports.EmailDeliveryRepository], m ports.EmailSender, v ports.Tokens) *CredentialDeliveryService {
	return &CredentialDeliveryService{u, m, v}
}
func (s *CredentialDeliveryService) Deliver(ctx context.Context) (bool, error) {
	if !s.mail.Ready() {
		return false, nil
	}
	var job domain.EmailJob
	e := s.unit.Within(ctx, func(ctx context.Context, r ports.EmailDeliveryRepository) error {
		var e error
		job, e = r.Claim(ctx, s.values.ID())
		return e
	})
	if e != nil || job.ID == "" {
		return false, e
	}
	delivered := job.Valid && s.mail.Send(ctx, job.Email.Recipient, job.Email.Subject, job.Email.Message) == nil
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.EmailDeliveryRepository) error {
		return r.Finish(ctx, job.ID, job.LeaseID, delivered)
	})
	if e == nil && !delivered {
		return true, ErrCredentialDelivery
	}
	return true, e
}
