// Package master composes Master services, routes and published providers.
package master

import (
	"context"
	inboxRepository "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	inboxContracts "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authContracts "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventsContracts "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	masterHTTP "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
	storage "github.com/ranakdinesh/epcERP/internal/modules/storage/adapters/outbound"
)

// New constructs the module services and binds its HTTP adapters.
func New(mux *infrastructure.Router, pool *pgxpool.Pool, identity authContracts.Resolver, journal func(database.Connection) eventsContracts.Writer, assignments func(database.Connection) authContracts.AssignmentDirectory) error {
	auth := infrastructure.NewAuthBridge(identity)
	inspector := infrastructure.NewDocumentInspector(masterDomain.VendorDocumentMaximumBytes)
	services := masterHTTP.Services{}
	cipher, e := infrastructure.NewDocumentCipher(os.Getenv("EPCERP_SUPPLIER_DOCUMENT_KEY"))
	if e != nil {
		return e
	}
	objects, bucket, e := DocumentObjects()
	if e != nil {
		return e
	}
	services.VendorReviewAssignments = masterApplication.NewVendorReviewAssignments(masterRepository.NewVendorReviewAssignmentUnitOfWork(pool, assignments, journal, func(c database.Connection) inboxContracts.SupplierReviewWriter {
		return inboxRepository.NewSupplierReviewWriter(c, journal(c), inboxRepository.Values{})
	}), auth, masterRepository.VendorDocumentValues{})
	approval := func(c database.Connection) masterPorts.DocumentApprovalLifecycle {
		return masterApplication.NewDocumentApprovalService(masterRepository.NewDocumentApprovalRepository(c, journal(c)), masterRepository.NewApprovalUserDirectory(assignments(c)))
	}
	services.Vendors = masterApplication.NewVendorService(masterRepository.NewVendorUnitOfWork(pool, journal), auth, masterRepository.VendorValues{})
	services.VendorDocuments = masterApplication.NewVendorDocumentService(masterRepository.NewVendorDocumentUnitOfWork(pool, func(c database.Connection) masterContracts.VendorDirectory {
		return masterRepository.NewVendorDirectory(c)
	}, journal).WithApprovalLifecycle(approval), auth, cipher, masterRepository.VendorDocumentValues{}).WithInspector(inspector).WithObjects(objects, bucket)
	services.VendorVerification = masterApplication.NewVendorVerificationService(masterRepository.NewVendorVerificationUnitOfWork(pool, func(c database.Connection) masterContracts.VendorDirectory {
		return masterRepository.NewVendorDirectory(c)
	}, func(c database.Connection) masterContracts.VendorDocumentReader {
		return masterRepository.NewVendorDocumentEvidenceReader(c, cipher, inspector).WithObjects(objects)
	}, journal, assignments).WithApprovalLifecycle(approval), auth, masterRepository.VendorVerificationValues{})
	services.Items = masterApplication.NewItemService(masterRepository.NewItemUnitOfWork(pool,
		func(c database.Connection) masterContracts.CategoryItemReader {
			return masterRepository.NewCategoryDirectory(c)
		},
		func(c database.Connection) masterContracts.TemplateItemReader {
			return masterRepository.NewTemplateItemReader(c)
		},
		func(c database.Connection) masterContracts.ManufacturerItemReader {
			return masterRepository.NewManufacturerItemReader(c)
		},
		journal), auth, masterRepository.ItemValues{})
	catalogue := func(c database.Connection) masterContracts.CategoryEligibilityReader {
		return masterRepository.NewCategoryDirectory(c)
	}
	services.Manufacturers = masterApplication.NewManufacturerService(masterRepository.NewManufacturerUnitOfWork(pool, catalogue, journal), auth, masterRepository.ManufacturerSystemValues{})
	services.Categories = masterApplication.NewCategoryService(masterRepository.NewCategoryUnitOfWork(pool, journal), auth, masterRepository.CategoryValues{})
	services.Templates = masterApplication.NewTemplateService(masterRepository.NewTemplateUnitOfWork(pool, func(c database.Connection) masterContracts.CategoryDirectory {
		return masterRepository.NewCategoryDirectory(c)
	}, journal), auth, masterRepository.TemplateValues{})
	services.References = masterApplication.NewReferenceAdministration(masterRepository.NewReferenceUnitOfWork(pool), auth, masterRepository.ReferenceSystemValues{})
	directory := func(c database.Connection) masterContracts.ReferenceReader {
		return masterApplication.NewReferenceService(masterRepository.NewReferenceRepository(c))
	}
	services.Clients = masterApplication.NewClientService(masterRepository.NewClientUnitOfWork(pool, directory), auth, masterRepository.ClientSystemValues{})

	masterHTTP.Register(mux, services)
	return nil
}

func ClientDirectory(connection database.Connection) masterContracts.ClientDirectory {
	return masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(connection))
}

func DocumentObjects() (*infrastructure.ObjectStore, string, error) {
	cfg, err := storage.LoadConfig(os.LookupEnv)
	if err != nil {
		return nil, "", err
	}
	objects, err := infrastructure.NewObjectStore(infrastructure.ObjectStoreConfig{Endpoint: cfg.Endpoint, AccessKey: cfg.AccessKey, SecretKey: cfg.SecretKey, Secure: cfg.Secure}, masterDomain.VendorDocumentMaximumBytes+28)
	return objects, cfg.Bucket, err
}

// DocumentEvidence is the published reader composition for downstream verification gates.
func DocumentEvidence(c database.Connection, cipher masterPorts.VendorDocumentCipher, inspector masterPorts.VendorDocumentInspector) masterContracts.VendorDocumentReader {
	objects, _, err := DocumentObjects()
	if err != nil {
		return failedDocumentReader{err}
	}
	return masterRepository.NewVendorDocumentEvidenceReader(c, cipher, inspector).WithObjects(objects)
}

type failedDocumentReader struct{ err error }

func (r failedDocumentReader) Active(context.Context, string) ([]masterContracts.VendorDocumentEvidence, error) {
	return nil, r.err
}

// RelocateDocuments is an offline operator action after migration 67, never startup behavior.
func RelocateDocuments(ctx context.Context, pool *pgxpool.Pool, journal func(database.Connection) eventsContracts.Writer) (int, error) {
	objects, bucket, err := DocumentObjects()
	if err != nil {
		return 0, err
	}
	cipher, err := infrastructure.NewDocumentCipher(os.Getenv("EPCERP_SUPPLIER_DOCUMENT_KEY"))
	if err != nil {
		return 0, err
	}
	return masterApplication.NewVendorDocumentRelocation(masterRepository.NewVendorDocumentRelocationUnitOfWork(pool, journal), objects, cipher, masterRepository.VendorDocumentValues{}, bucket).Run(ctx)
}

func ReviewTasks(c database.Connection, users authContracts.AssignmentDirectory) masterContracts.VendorReviewTasks {
	return masterRepository.NewVendorReviewTasks(c, users, nil, nil)
}

func WorkInventory(c database.Connection) authContracts.WorkSource {
	return masterRepository.NewWorkInventory(c)
}
