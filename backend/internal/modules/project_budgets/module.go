package project_budgets

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	workprojects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	authout "github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository"
	authapp "github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	inboxout "github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound"
	eventsout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventsapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	financeout "github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/adapters/outbound"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	govout "github.com/ranakdinesh/epcERP/internal/modules/organisation_governance/adapters/outbound"
	accessmodule "github.com/ranakdinesh/epcERP/internal/modules/project_access"
	accessout "github.com/ranakdinesh/epcERP/internal/modules/project_access/adapters/outbound"
	in "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/adapters/inbound"
	out "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/adapters/outbound"
	app "github.com/ranakdinesh/epcERP/internal/modules/project_budgets/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/ports"
	projectout "github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound"
	purchaseRepository "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository"
)

func New(r chi.Router, pool *pgxpool.Pool, identity auth.Resolver) {
	providers := func(c database.Connection) out.Providers {
		return out.Providers{Scope: accessout.NewGate(c, accessmodule.Providers(c)), Projects: projectout.NewContextReader(c), Clients: masterApplication.NewClientDirectory(masterRepository.NewClientIdentityReader(c)), BO: purchaseRepository.NewBOReader(c)}
	}
	journal := func(c database.Connection) events.Writer { return eventsapp.NewRecorder(eventsout.NewJournal(c)) }
	finance := func(c database.Connection) ports.Approvals {
		users := authapp.NewAssignmentDirectory(authout.NewAssignmentReader(c))
		notices := inboxout.NewWriter(c, govout.NewDirectory(c, users), journal(c), inboxout.Values{})
		return out.NewFinanceBridge(financeout.NewBudgetApprover(c, users, accessout.NewGate(c, accessmodule.Providers(c)), journal(c), notices, financeout.Values{}))
	}
	in.Register(r, app.New(out.NewFinanceUnitOfWork(pool, providers, journal, finance), out.NewAuthBridge(identity), out.Values{}))
}

func WorkInventory(c database.Connection, parents workprojects.ContextReader) auth.WorkSource {
	return out.NewWorkInventory(c, parents)
}
