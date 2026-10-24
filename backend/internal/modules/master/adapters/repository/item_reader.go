package repository

import (
	"context"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"

	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ItemReader struct {
	repo    *itemRepository
	context *itemItemContext
}

func NewItemReader(c database.Connection, categories masterContracts.CategoryItemReader, templates masterContracts.TemplateItemReader, makes masterContracts.ManufacturerItemReader) *ItemReader {
	return &ItemReader{&itemRepository{q: NewStore(c).Queries}, &itemItemContext{categories, templates, makes}}
}
func (r *ItemReader) Get(ctx context.Context, id string) (masterDomain.Item, error) {
	return r.repo.Get(ctx, id)
}
func (r *ItemReader) List(ctx context.Context) ([]masterDomain.Item, error) { return r.repo.List(ctx) }
func (r *ItemReader) Approved(ctx context.Context, id string) (masterDomain.Item, error) {
	item, err := r.repo.Get(ctx, id)
	if err != nil {
		return item, err
	}
	if item.Status != "approved" || !item.Active {
		return item, masterDomain.ItemInvalid("item_id", "Only active approved catalogue items may be selected")
	}
	config, err := r.context.Configuration(ctx, item.ConfigurationID)
	if err != nil {
		return item, err
	}
	make, err := r.context.Manufacturer(ctx, item.ManufacturerID)
	if err != nil {
		return item, err
	}
	if !masterDomain.ItemEligible(config, make) {
		return item, masterDomain.ItemInvalid("item_id", "Item configuration, category, unit or manufacturer is inactive or ineligible")
	}
	return item, nil
}
