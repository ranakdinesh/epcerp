package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/ports"
)

func canReadProject(ctx context.Context, r ports.Repository, a domain.Principal, p domain.Project) (bool, error) {
	if domain.CanRead(a, p) {
		return true, nil
	}
	if scoped, ok := r.(ports.ScopedReadRepository); ok {
		return scoped.CanReadAssigned(ctx, a, p.ID)
	}
	return false, nil
}
