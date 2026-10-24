// Package document_packages owns source receipts, quarantine and bounded validation.
package document_packages

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/adapters/inbound"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/application"
	"github.com/ranakdinesh/epcERP/internal/modules/document_packages/contracts"
	reading "github.com/ranakdinesh/epcERP/internal/modules/document_reading/contracts"
	"net/http"
	"os"
)

type Dependencies struct {
	Router   *infrastructure.Router
	Database *pgxpool.Pool
	Identity auth.Resolver
	Users    func(database.Connection) auth.AssignmentDirectory
	Preparer reading.Preparer
}
type Module struct {
	Objects outbound.Objects
	Worker  *application.Worker
	Store   *infrastructure.DocumentStreamStorage
}

func New(d Dependencies) (*Module, error) {
	cipher, e := infrastructure.NewDocumentCipher(os.Getenv("EPCERP_DOCUMENT_READING_KEY"))
	if e != nil {
		return nil, e
	}
	dir := os.Getenv("EPCERP_DOCUMENT_PACKAGE_ROOT")
	if dir == "" {
		dir = "var/document-packages"
	}
	store, e := infrastructure.NewDocumentStreamStorage(dir, cipher)
	if e != nil {
		return nil, e
	}
	unit := outbound.NewUnit(d.Database, d.Users)
	service := application.New(unit, outbound.Auth{Resolver: d.Identity}, outbound.Values{}, outbound.Objects{Store: store}).WithLimits(infrastructure.NewReceiptLimits())
	if d.Router != nil {
		inbound.Register(d.Router, service)
		d.Router.AddPayloadPolicy(func(r *http.Request) bool { return r.Method == "PUT" && inbound.IsUploadPath(r.URL.Path) }, (64<<20)+1, nil)
	}
	worker := application.NewWorker(unit, outbound.Values{}, outbound.Verifier{Store: store, Scanner: outbound.ClamdScanner{Socket: os.Getenv("EPCERP_CLAMD_SOCKET")}, Pages: outbound.PDFInfo{}, Preparer: d.Preparer})
	return &Module{Objects: outbound.Objects{Store: store}, Worker: worker, Store: store}, nil
}
func Consumer(c database.Connection, u auth.AssignmentDirectory) contracts.Consumer {
	return application.NewConsumer(outbound.NewRepository(c, u))
}

func DraftConsumer(c database.Connection, u auth.AssignmentDirectory) contracts.DraftConsumer {
	return application.NewConsumer(outbound.NewRepository(c, u))
}
