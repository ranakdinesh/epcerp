// Package tender_fms exposes the Tender FMS module composition boundary.
package tender_fms

import (
	notices "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	packages "github.com/ranakdinesh/epcERP/internal/modules/document_packages/contracts"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/inbound"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/application"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/ports"
)

type Dependencies struct {
	Packages         func(database.Connection) packages.Consumer
	PreparedReader   func(database.Connection) reading.PreparedReader
	Notices          func(database.Connection) notices.TenderWriter
	Bundles          func(database.Connection) auth.BundleDirectory
	ManagerBundleIDs []string
	Users            func(database.Connection) auth.AssignmentDirectory
	Router           *infrastructure.Router
	Database         *pgxpool.Pool
	Identity         auth.Resolver
	Reader           func(database.Connection) reading.Reader
	Validator        reading.Validator
	Events           func(database.Connection) events.Writer
}

type Module struct{}

func New(dependencies Dependencies) *Module {
	documents := func(connection database.Connection) ports.Documents {
		return outbound.NewDocuments(dependencies.Reader(connection), dependencies.Validator)
	}
	service := application.New(outbound.NewUnitOfWork(dependencies.Database, documents, dependencies.Events).WithUsers(dependencies.Users), outbound.NewAuthBridge(dependencies.Identity), outbound.Values{})
	inbound.Register(dependencies.Router, service)
	if dependencies.Packages != nil && dependencies.PreparedReader != nil && dependencies.Notices != nil {
		foundation := func(c database.Connection) contracts.Records {
			return Records(c, dependencies.Users(c), dependencies.Bundles(c), dependencies.ManagerBundleIDs, dependencies.Events(c))
		}
		unit := outbound.NewCaptureUnit(dependencies.Database, dependencies.Users, dependencies.Bundles, dependencies.ManagerBundleIDs, dependencies.Events, foundation, dependencies.Packages, dependencies.PreparedReader, dependencies.Notices)
		inbound.RegisterCapture(dependencies.Router, application.NewCapture(unit, outbound.NewAuthBridge(dependencies.Identity), outbound.Values{}).WithLimits(infrastructure.NewTenderLimits()))
	}
	inbound.RegisterManagerOptions(dependencies.Router, outbound.NewManagerOptions(dependencies.Database, dependencies.Identity, func(c database.Connection) contracts.Records {
		return Records(c, dependencies.Users(c), dependencies.Bundles(c), dependencies.ManagerBundleIDs, dependencies.Events(c))
	}))
	inbound.RegisterOwnership(dependencies.Router, application.NewOwnership(outbound.NewOwnershipUnit(dependencies.Database, dependencies.Users, dependencies.Events), outbound.NewAuthBridge(dependencies.Identity), outbound.Values{}))
	dependencies.Router.AddPayloadPolicy(func(request *http.Request) bool {
		return request.Method == http.MethodPost && inbound.IsUploadPath(request.URL.Path)
	}, inbound.UploadBytes+(64<<10), inbound.UploadGuard(service))
	return &Module{}
}

func WorkInventory(c database.Connection) auth.WorkSource { return outbound.NewWorkInventory(c) }

// Records binds foundational record operations to the caller's transaction. It does
// not start a transaction, send notifications or provision a successful capture.
func Records(c database.Connection, u auth.AssignmentDirectory, b auth.BundleDirectory, approved []string, j events.Writer) contracts.Records {
	return application.NewRecords(outbound.NewRecordsRepository(c, j), outbound.NewEligibleUsers(u, b, approved), outbound.Values{})
}

func AssignmentActions(c database.Connection, j events.Writer) contracts.AssignmentActions {
	return outbound.NewCaptureRepository(c, j)
}
