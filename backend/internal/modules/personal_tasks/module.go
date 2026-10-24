package personal_tasks

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	master "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	in "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/personal_tasks/application"
	purchase "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
)

type Dependencies struct {
	SupplierReviews func(database.Connection) master.VendorReviewTasks
	Router          chi.Router
	Database        *pgxpool.Pool
	Identity        auth.Resolver
	Users           func(database.Connection) auth.AssignmentDirectory
	Quotations      func(database.Connection) purchase.QuotationWorkReader
	Workflow        func(database.Connection) purchase.WorkflowTaskReader
}

func New(dependencies Dependencies) {
	in.Register(dependencies.Router, app.New(out.NewUnitOfWork(dependencies.Database, func(connection database.Connection) out.Providers {
		return out.Providers{
			SupplierReviews: supplierReviewProvider(dependencies, connection),
			Quotes:          dependencies.Quotations(connection),
			Users:           dependencies.Users(connection),
			Purchase:        dependencies.Workflow(connection),
		}
	}), out.NewAuthBridge(dependencies.Identity), out.Values{}))
}

func supplierReviewProvider(d Dependencies, c database.Connection) master.VendorReviewTasks {
	if d.SupplierReviews == nil {
		return nil
	}
	return d.SupplierReviews(c)
}
