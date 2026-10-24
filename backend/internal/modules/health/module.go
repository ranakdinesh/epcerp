// Package health exposes the health capability as an independently composed module.
package health

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/modules/health/adapters/inbound"
	"github.com/ranakdinesh/epcERP/internal/modules/health/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/health/application"
)

type Module struct{}

func New(router chi.Router, database *pgxpool.Pool) *Module {
	inbound.Register(router, application.New(outbound.NewPostgresProbe(database)))
	return &Module{}
}
