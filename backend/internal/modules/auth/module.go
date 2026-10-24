package auth

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/helpers"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/logging"
	httpx "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/httpx"
	authIdentity "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/identity"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/jobs"
	repository "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	security "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/security"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	eventsContracts "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	storage "github.com/ranakdinesh/epcERP/internal/modules/storage/adapters/outbound"
)

type Dependencies struct {
	Router   *infrastructure.Router
	Database *pgxpool.Pool
	Logger   logging.Logger
	Events   func(database.Connection) eventsContracts.Writer
	Email    *infrastructure.EmailService
	Work     func(database.Connection) map[string]contracts.WorkSource
}

type Module struct {
	Resolver contracts.Resolver
}

func (m *Module) Assignments(connection database.Connection) contracts.AssignmentDirectory {
	return application.NewAssignmentDirectory(repository.NewAssignmentReader(connection))
}

func (m *Module) Bundles(connection database.Connection) contracts.BundleDirectory {
	return repository.NewBundleDirectory(connection)
}

func New(ctx context.Context, dependencies Dependencies) (*Module, error) {
	cfg, err := security.LoadAuthConfig(os.LookupEnv)
	if err != nil {
		return nil, err
	}
	passwords, err := security.NewPasswords()
	if err != nil {
		return nil, err
	}
	tokens, err := security.NewTokens(cfg.Tokens)
	if err != nil {
		return nil, err
	}
	pool := dependencies.Database
	bootstrap := application.NewBootstrap(repository.NewBootstrapUnitOfWork(pool), passwords, tokens)
	if _, err = bootstrap.Bootstrap(ctx, cfg.BootstrapUsername, cfg.BootstrapEmail, cfg.BootstrapPassword); err != nil {
		return nil, err
	}
	dependencies.Logger.Info().Str("event", "auth.bootstrap.completed").Msg("Auth registry and recovery identity reconciled")
	mail := security.NewRecoveryMail(dependencies.Email, dependencies.Email.PublicURL)
	emailCipher, err := infrastructure.NewEmailCipher(os.Getenv("EPCERP_IMAGE_ENCRYPTION_KEY"))
	if err != nil {
		return nil, err
	}
	credentialUnit := repository.NewCredentialUnitOfWork(pool, dependencies.Events, emailCipher)
	credentials := application.NewCredentials(credentialUnit, passwords, tokens, security.TemporaryPasswords{}, dependencies.Email, dependencies.Email.PublicURL+"/login")
	delivery := application.NewCredentialDelivery(repository.NewCredentialDeliveryUnitOfWork(pool, emailCipher), dependencies.Email, tokens)

	sessions := application.NewSessions(repository.NewSessionUnitOfWork(pool), passwords, tokens, cfg.Policy)
	services := httpx.Services{
		Offboarding:  application.NewOffboarding(repository.NewOffboardingUnitOfWork(pool, dependencies.Events, dependencies.Work), tokens),
		Organisation: application.NewOrganisation(repository.NewOrganisationUnitOfWork(pool, dependencies.Events, dependencies.Work), tokens),
		Credentials:  credentials,
		Recovery:     application.NewRecovery(repository.NewRecoveryUnitOfWork(pool, dependencies.Events), passwords, tokens, mail),
		Sessions:     sessions,
		Users:        application.NewUsers(repository.NewUserUnitOfWork(pool), passwords, tokens, cfg.Policy),
		Roles:        application.NewRoles(repository.NewRoleUnitOfWork(pool), tokens),
		Account:      application.NewAccount(repository.NewAccountUnitOfWork(pool, dependencies.Events), passwords, tokens),
	}
	identity := authIdentity.NewIdentityResolver(sessions)
	storageConfig, err := storage.LoadConfig(os.LookupEnv)
	if err != nil {
		return nil, err
	}
	objects, err := infrastructure.NewObjectStore(infrastructure.ObjectStoreConfig{Endpoint: storageConfig.Endpoint, AccessKey: storageConfig.AccessKey, SecretKey: storageConfig.SecretKey, Secure: storageConfig.Secure}, domain.ProfileMaximumPhotoBytes+28)
	if err != nil {
		return nil, err
	}
	cipher, err := infrastructure.NewDocumentCipher(os.Getenv("EPCERP_IMAGE_ENCRYPTION_KEY"))
	if err != nil {
		return nil, err
	}
	images, err := infrastructure.NewImageStorage(objects, cipher, storageConfig.Bucket, domain.ProfileMaximumPhotoBytes)
	if err != nil {
		return nil, err
	}
	services.Profile = application.NewProfile(repository.NewProfileUnitOfWork(pool, dependencies.Events), infrastructure.NewAuthBridge(identity), helpers.ImageProcessor{MaximumBytes: domain.ProfileMaximumPhotoBytes, MaximumDimension: 2048, Size: 256}, tokens, images)
	httpx.Register(dependencies.Router, services)
	go jobs.RunCredentialMail(ctx, delivery, dependencies.Logger)
	return &Module{Resolver: identity}, nil
}
