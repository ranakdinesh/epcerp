package repository

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
)

func unique(err error) bool {
	var postgres *pgconn.PgError
	return errors.As(err, &postgres) && postgres.Code == "23505"
}

type unitOfWork[R any] struct {
	work    func(database.Connection) map[string]contracts.WorkSource
	manager *database.Manager
	events  func(database.Connection) events.Writer
	bind    func(*Store) R
}

func (u *unitOfWork[R]) Within(ctx context.Context, work func(context.Context, R) error) error {
	return u.manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		var journal events.Writer
		if u.events != nil {
			journal = u.events(c)
		}
		store := NewStore(c, journal)
		if u.work != nil {
			store.work = u.work(c)
		}
		return work(ctx, u.bind(store))
	})
}
func NewSessionUnitOfWork(b database.Beginner) ports.UnitOfWork[ports.SessionRepository] {
	return &unitOfWork[ports.SessionRepository]{manager: database.NewManager(b), bind: func(s *Store) ports.SessionRepository { return &sessionRepository{s} }}
}
func NewUserUnitOfWork(b database.Beginner) ports.UnitOfWork[ports.UserRepository] {
	return &unitOfWork[ports.UserRepository]{manager: database.NewManager(b), bind: func(s *Store) ports.UserRepository { return &userRepository{s} }}
}
func NewRoleUnitOfWork(b database.Beginner) ports.UnitOfWork[ports.RoleRepository] {
	return &unitOfWork[ports.RoleRepository]{manager: database.NewManager(b), bind: func(s *Store) ports.RoleRepository { return &roleRepository{s} }}
}
func NewBootstrapUnitOfWork(b database.Beginner) ports.UnitOfWork[ports.BootstrapRepository] {
	return &unitOfWork[ports.BootstrapRepository]{manager: database.NewManager(b), bind: func(s *Store) ports.BootstrapRepository { return &bootstrapRepository{s} }}
}
func NewAccountUnitOfWork(b database.Beginner, e func(database.Connection) events.Writer) ports.UnitOfWork[ports.AccountRepository] {
	return &unitOfWork[ports.AccountRepository]{manager: database.NewManager(b), events: e, bind: func(s *Store) ports.AccountRepository { return &accountRepository{s} }}
}

func NewOrganisationUnitOfWork(b database.Beginner, e func(database.Connection) events.Writer, providers ...func(database.Connection) map[string]contracts.WorkSource) ports.UnitOfWork[ports.OrganisationRepository] {
	var work func(database.Connection) map[string]contracts.WorkSource
	if len(providers) > 0 {
		work = providers[0]
	}
	return &unitOfWork[ports.OrganisationRepository]{work: work, manager: database.NewManager(b), events: e, bind: func(s *Store) ports.OrganisationRepository { return &organisationRepository{s} }}
}

func NewProfileUnitOfWork(b database.Beginner, e func(database.Connection) events.Writer) ports.UnitOfWork[ports.ProfileRepository] {
	return &unitOfWork[ports.ProfileRepository]{manager: database.NewManager(b), events: e, bind: func(s *Store) ports.ProfileRepository { return &profileRepository{s} }}
}
