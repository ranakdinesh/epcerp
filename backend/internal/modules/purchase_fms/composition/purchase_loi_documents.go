package composition

import (
	"github.com/jackc/pgx/v5/pgxpool"
	infra "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	master "github.com/ranakdinesh/epcERP/internal/modules/master"
	httpx "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/httpx"
	repo "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
	app "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/application"
)

func registerLOIDocuments(s *httpx.Services, pool *pgxpool.Pool, a auth.Resolver, f repo.IndentsFactory, j func(database.Connection) events.Writer, cipher *infra.DocumentCipher) error {
	objects, bucket, e := master.DocumentObjects()
	if e != nil {
		return e
	}
	u := repo.NewAwardsUnitOfWork(pool, func(c database.Connection) repo.AwardsProviders {
		p := f(c)
		return repo.AwardsProviders{Indents: p, Workflow: purchaseWorkflowProviders(c, p), Sourcing: purchaseSourcingProviders(c, p, cipher), Approvals: quotationApprovalProviders(c, cipher)}
	}, j)
	s.LOIDocuments = app.NewLOIDocuments(u, repo.NewAuthBridge(a), repo.Values{}, repo.LOIPDFPrinter{}, repo.LOIFingerprint{}, cipher, objects, bucket)
	return nil
}
