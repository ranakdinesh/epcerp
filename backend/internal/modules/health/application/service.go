package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/health/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/health/ports"
)

type Service struct{ database ports.DatabaseProbe }

func New(database ports.DatabaseProbe) *Service {
	if database == nil {
		panic("health: database probe is required")
	}
	return &Service{database: database}
}

func (s *Service) Live() domain.Status { return domain.OK }

func (s *Service) Ready(ctx context.Context) domain.Status {
	if err := s.database.Check(ctx); err != nil {
		return domain.Unavailable
	}
	return domain.OK
}
