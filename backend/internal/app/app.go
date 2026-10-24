// Package app owns platform initialization and binds independently composed modules.
package app

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/config"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/logging"
	authmodule "github.com/ranakdinesh/epcERP/internal/modules/auth"
	authContracts "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	inboxmodule "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox"
	inboxcontracts "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	packagemodule "github.com/ranakdinesh/epcERP/internal/modules/document_packages"
	packagecontracts "github.com/ranakdinesh/epcERP/internal/modules/document_packages/contracts"
	readingmodule "github.com/ranakdinesh/epcERP/internal/modules/document_reading"
	eventmodule "github.com/ranakdinesh/epcERP/internal/modules/event_delivery"
	accountsmodule "github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance"
	approvalmodule "github.com/ranakdinesh/epcERP/internal/modules/finance_approvals"
	healthmodule "github.com/ranakdinesh/epcERP/internal/modules/health"
	mastermodule "github.com/ranakdinesh/epcERP/internal/modules/master"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	governancemodule "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance"
	taskmodule "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks"
	accessmodule "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	budgetmodule "github.com/ranakdinesh/epcERP/internal/modules/project_budgets"
	itemmodule "github.com/ranakdinesh/epcERP/internal/modules/project_items"
	projectmodule "github.com/ranakdinesh/epcERP/internal/modules/projects"
	purchasemodule "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms"
	purchasecontracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	storagemodule "github.com/ranakdinesh/epcERP/internal/modules/storage"
	tendermodule "github.com/ranakdinesh/epcERP/internal/modules/tender_fms"
	calendarmodule "github.com/ranakdinesh/epcERP/internal/modules/work_calendar"
)

type App struct {
	PackageStore *infrastructure.DocumentStreamStorage
	Logger       logging.Logger
	Config       config.Config
	Router       *infrastructure.Router
	Database     *pgxpool.Pool
	HTTPServer   *infrastructure.HTTPServer
}

func New(ctx context.Context) (*App, error) {
	configuration, logger, err := loadPlatform()
	if err != nil {
		return nil, err
	}
	pool, err := infrastructure.OpenDatabase(ctx, configuration)
	if err != nil {
		logger.Error().Str("event", "database.unavailable").Msg("Database initialization failed; check configuration and migrations")
		return nil, err
	}
	router := infrastructure.NewRouter(configuration, logger)
	runtime := &App{Logger: logger, Config: configuration, Router: router, Database: pool}
	if err = runtime.bindModules(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	runtime.HTTPServer = infrastructure.NewHTTPServer(configuration.HTTPAddress, router, logger)
	return runtime, nil
}

func loadPlatform() (config.Config, logging.Logger, error) {
	if err := config.LoadLocalEnvironment(); err != nil {
		logger, _ := infrastructure.NewLogger("info")
		logger.Error().Str("event", "config.invalid").Msg("Invalid local development configuration")
		return config.Config{}, logger, err
	}
	configuration, err := config.Load(os.LookupEnv)
	if err != nil {
		logger, _ := infrastructure.NewLogger("info")
		logger.Error().Str("event", "config.invalid").Msg("Invalid platform configuration")
		return config.Config{}, logger, err
	}
	logger, err := infrastructure.NewLogger(configuration.LogLevel)
	return configuration, logger, err
}

func (a *App) bindModules(ctx context.Context) error {
	email, err := infrastructure.NewEmailService(os.LookupEnv)
	if err != nil {
		return err
	}
	events := eventmodule.New()
	auth, err := authmodule.New(ctx, authmodule.Dependencies{Router: a.Router, Database: a.Database, Logger: a.Logger, Events: events.Writer, Email: email, Work: func(c database.Connection) map[string]authContracts.WorkSource {
		return map[string]authContracts.WorkSource{
			"projects": projectmodule.WorkInventory(c), "purchase_fms": purchasemodule.WorkInventory(c, projectmodule.ContextReader(c)), "master": mastermodule.WorkInventory(c), "project_access": accessmodule.WorkInventory(c, projectmodule.ContextReader(c)), "project_budgets": budgetmodule.WorkInventory(c, projectmodule.ContextReader(c)), "organisation_governance": governancemodule.WorkInventory(c), "tender_fms": tendermodule.WorkInventory(c),
		}
	}})
	if err != nil {
		return err
	}
	healthmodule.New(a.Router, a.Database)
	approvalmodule.New()
	reader, err := readingmodule.New(a.Database, events.Writer)
	if err != nil {
		return err
	}
	packages, err := packagemodule.New(packagemodule.Dependencies{Router: a.Router, Database: a.Database, Identity: auth.Resolver, Users: auth.Assignments, Preparer: reader.Preparer()})
	if err != nil {
		return err
	}
	a.PackageStore = packages.Store
	tendermodule.New(tendermodule.Dependencies{DraftPackages: func(c database.Connection) packagecontracts.DraftConsumer {
		return packagemodule.DraftConsumer(c, auth.Assignments(c))
	}, Packages: func(c database.Connection) packagecontracts.Consumer {
		return packagemodule.Consumer(c, auth.Assignments(c))
	}, PreparedReader: reader.PreparedReader, Notices: func(c database.Connection) inboxcontracts.TenderWriter {
		return inboxmodule.TenderWriter(c, events.Writer(c), tendermodule.AssignmentActions(c, events.Writer(c)))
	}, Bundles: auth.Bundles, ManagerBundleIDs: a.Config.TenderManagerBundleIDs, Users: auth.Assignments, Router: a.Router, Database: a.Database, Identity: auth.Resolver, Reader: reader.Reader, Validator: reader.Validator(), Events: events.Writer})
	if err = mastermodule.New(a.Router, a.Database, auth.Resolver, events.Writer, auth.Assignments); err != nil {
		return err
	}
	accessmodule.New(a.Router, a.Database, auth.Resolver)
	budgetmodule.New(a.Router, a.Database, auth.Resolver)
	governancemodule.New(a.Router, a.Database, auth.Resolver)
	inboxmodule.New(a.Router, a.Database, auth.Resolver)
	purchase, err := purchasemodule.New(a.Router, a.Database, auth.Resolver)
	if err != nil {
		return err
	}
	taskmodule.New(taskmodule.Dependencies{SupplierReviews: func(c database.Connection) masterContracts.VendorReviewTasks {
		return mastermodule.ReviewTasks(c, auth.Assignments(c))
	}, Router: a.Router, Database: a.Database, Identity: auth.Resolver, Users: purchase.TaskProviders.Users, Quotations: purchase.TaskProviders.Quotations, Workflow: purchase.TaskProviders.Workflow})
	itemmodule.New(itemmodule.Dependencies{Router: a.Router, Database: a.Database, Identity: auth.Resolver, Providers: purchase.ProjectItems, Events: events.Writer})
	accountsmodule.New(accountsmodule.Dependencies{Router: a.Router, Database: a.Database, Identity: auth.Resolver, Providers: purchase.Accounts, Events: events.Writer})
	calendarmodule.New(calendarmodule.Dependencies{
		Router: a.Router, Database: a.Database, Identity: auth.Resolver, Events: events.Writer,
		Refresh: func(connection database.Connection, reader calendarmodule.ReaderContract) func(context.Context, string) (int, error) {
			return projectmodule.CalendarUpdater(connection, events.Writer(connection), reader)
		},
	})
	projectmodule.New(projectmodule.Dependencies{
		Router: a.Router, Database: a.Database, Identity: auth.Resolver,
		Clients: mastermodule.ClientDirectory, Assignments: auth.Assignments, Events: events.Writer, Calendar: calendarmodule.Reader,
		Purchasing: func(connection database.Connection) purchasecontracts.WorkspaceOwnership {
			return purchasemodule.WorkspaceOwnership(connection, auth.Assignments(connection), events.Writer(connection))
		},
		ReadableAssignments: accessmodule.ReadableProjects,
	})
	_, err = storagemodule.New(a.Router, a.Database, auth.Resolver)
	return err
}

func (a *App) Run(ctx context.Context) error { return a.HTTPServer.Run(ctx) }

func (a *App) Close() {
	if a.PackageStore != nil {
		_ = a.PackageStore.Close()
	}
	a.Database.Close()
}
