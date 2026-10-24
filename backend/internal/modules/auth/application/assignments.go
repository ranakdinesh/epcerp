package application

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type AssignmentDirectory struct{ reader ports.AssignmentReader }

func NewAssignmentDirectory(r ports.AssignmentReader) *AssignmentDirectory {
	return &AssignmentDirectory{r}
}
func (d *AssignmentDirectory) Get(ctx context.Context, id string) (*domain.AssignmentCandidate, error) {
	return d.reader.Assignment(ctx, id)
}
func (d *AssignmentDirectory) List(ctx context.Context) ([]domain.AssignmentCandidate, error) {
	return d.reader.Assignments(ctx)
}
