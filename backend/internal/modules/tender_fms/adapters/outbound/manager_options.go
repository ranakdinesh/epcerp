package outbound

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpmiddleware"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"time"
)

type ManagerOptions struct {
	limiter *httpmiddleware.Limiter
	m       *database.Manager
	auth    auth.Resolver
	records func(database.Connection) contracts.Records
}

func NewManagerOptions(b database.Beginner, a auth.Resolver, r func(database.Connection) contracts.Records) *ManagerOptions {
	return &ManagerOptions{limiter: httpmiddleware.NewLimiter(120, time.Minute), m: database.NewManager(b), auth: a, records: r}
}
func (s *ManagerOptions) Options(ctx context.Context, token string) ([]domain.Candidate, error) {
	a, e := s.auth.Resolve(ctx, token)
	if e != nil {
		return nil, e
	}
	if a.AccountNotReady {
		return nil, domain.Forbidden
	}
	if !a.Active || a.ID == "" {
		return nil, domain.Unauthorized
	}
	if !s.limiter.Allow(a.ID) {
		return nil, &domain.Error{Status: 429, Detail: "Too many Tender Manager option requests"}
	}
	var out []domain.Candidate
	e = s.m.Within(ctx, func(ctx context.Context, c database.Connection) error {
		var e error
		out, e = s.records(c).ManagerOptions(ctx, a.ID)
		return e
	})
	return out, e
}
