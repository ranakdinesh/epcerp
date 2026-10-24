package repository

import (
	"context"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
)

type TemplateItemReader struct{ q *db.Queries }

func NewTemplateItemReader(c database.Connection) *TemplateItemReader {
	return &TemplateItemReader{NewStore(c).Queries}
}
func (r *TemplateItemReader) GetRevision(ctx context.Context, id string) (masterContracts.Template, error) {
	v, e := r.q.TemplatesGetTemplateByID(ctx, masterDTO.UUID(id))
	if e != nil {
		return masterContracts.Template{}, e
	}
	return (&templateRepository{q: r.q}).template(ctx, v)
}
func (r *TemplateItemReader) ListLatest(ctx context.Context) ([]masterContracts.Template, error) {
	rows, e := r.q.TemplatesListLatestTemplates(ctx)
	if e != nil {
		return nil, e
	}
	out := []masterContracts.Template{}
	for _, v := range rows {
		t, e := (&templateRepository{q: r.q}).template(ctx, v)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, nil
}
