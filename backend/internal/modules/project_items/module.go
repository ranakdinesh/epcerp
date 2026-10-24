// Package project_items exposes project material requirements as a composed module.
package project_items

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/inbound"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/application"
)

type Dependencies struct {
	Router    chi.Router
	Database  *pgxpool.Pool
	Identity  auth.Resolver
	Providers func(database.Connection) outbound.Providers
	Events    func(database.Connection) events.Writer
}

type Module struct{}

func New(dependencies Dependencies) *Module {
	service := application.New(outbound.NewUnitOfWork(dependencies.Database, dependencies.Providers, dependencies.Events), outbound.NewAuthBridge(dependencies.Identity), outbound.Values{})
	inbound.Register(dependencies.Router, service)
	return &Module{}
}
