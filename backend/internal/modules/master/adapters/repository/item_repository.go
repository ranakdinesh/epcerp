package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"
	"strings"
	"unicode"
	"unicode/utf16"

	pgx "github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type ItemUnitOfWork struct {
	manager    *database.Manager
	categories func(database.Connection) masterContracts.CategoryItemReader
	templates  func(database.Connection) masterContracts.TemplateItemReader
	makes      func(database.Connection) masterContracts.ManufacturerItemReader
	journal    func(database.Connection) events.Writer
}

func NewItemUnitOfWork(b database.Beginner, c func(database.Connection) masterContracts.CategoryItemReader, t func(database.Connection) masterContracts.TemplateItemReader, m func(database.Connection) masterContracts.ManufacturerItemReader, j func(database.Connection) events.Writer) *ItemUnitOfWork {
	return &ItemUnitOfWork{database.NewManager(b), c, t, m, j}
}
func (u *ItemUnitOfWork) Within(ctx context.Context, f func(context.Context, masterPorts.ItemRepository, masterPorts.ItemContext) error) error {
	return u.manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return f(ctx, &itemRepository{NewStore(c).Queries, u.journal(c)}, &itemItemContext{u.categories(c), u.templates(c), u.makes(c)})
	})
}

type itemRepository struct {
	q *db.Queries
	j events.Writer
}

func (r *itemRepository) Lock(ctx context.Context) error { return r.q.ItemsLockItems(ctx) }
func (r *itemRepository) List(ctx context.Context) ([]masterDomain.Item, error) {
	rows, e := r.q.ItemsListItems(ctx)
	if e != nil {
		return nil, e
	}
	out := []masterDomain.Item{}
	for _, v := range rows {
		x, e := masterDTO.ItemFromRow(v)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, nil
}
func (r *itemRepository) Get(ctx context.Context, id string) (masterDomain.Item, error) {
	v, e := r.q.ItemsGetItem(ctx, masterDTO.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return masterDomain.Item{}, masterDomain.ItemMissing
	}
	if e != nil {
		return masterDomain.Item{}, e
	}
	return masterDTO.ItemFromRow(v)
}
func (r *itemRepository) Find(ctx context.Context, key string) (*masterDomain.Item, error) {
	v, e := r.q.ItemsFindItem(ctx, key)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	x, e := masterDTO.ItemFromRow(v)
	return &x, e
}
func (r *itemRepository) Candidates(ctx context.Context, identity masterDomain.ItemIdentity) ([]masterDomain.Item, error) {
	spec, err := masterDTO.MarshalMasterJSON(identity.Specifications)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ItemsIdentityCandidates(ctx, db.ItemsIdentityCandidatesParams{ManufacturerID: masterDTO.UUID(identity.Manufacturer), Specifications: spec, Family: identity.Family, Uom: identity.UOM})
	if err != nil {
		return nil, err
	}
	result := []masterDomain.Item{}
	for _, row := range rows {
		item, err := masterDTO.ItemFromRow(row)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}
func (r *itemRepository) Next(ctx context.Context, category string) (int32, error) {
	return r.q.ItemsNextSerial(ctx, category)
}
func (r *itemRepository) Create(ctx context.Context, v masterDomain.Item, actor string) (masterDomain.Item, error) {
	spec, e := masterDTO.MarshalMasterJSON(v.Specifications)
	if e != nil {
		return v, e
	}
	snapshot, e := masterDTO.MarshalMasterJSON(v.Snapshot)
	if e != nil {
		return v, e
	}
	row, e := r.q.ItemsCreateItem(ctx, db.ItemsCreateItemParams{ID: masterDTO.UUID(v.ID), ConfigurationID: masterDTO.UUID(v.ConfigurationID), ManufacturerID: masterDTO.UUID(v.ManufacturerID), CategoryID: masterDTO.UUID(v.CategoryID), Model: v.Model, Specifications: spec, Snapshot: snapshot, Fingerprint: v.Fingerprint, EngineeringKey: v.EngineeringKey, Description: v.Description, Sku: v.SKU, Serial: v.Serial, ActorID: masterDTO.UUID(actor)})
	if e != nil {
		return v, e
	}
	return masterDTO.ItemFromRow(row)
}
func (r *itemRepository) Change(ctx context.Context, v masterDomain.Item, actor string) (masterDomain.Item, error) {
	row, e := r.q.ItemsChangeItem(ctx, db.ItemsChangeItemParams{ID: masterDTO.UUID(v.ID), ExpectedVersion: v.Version, Status: v.Status, IsActive: v.Active, ActorID: masterDTO.UUID(actor)})
	if errors.Is(e, pgx.ErrNoRows) {
		return v, masterDomain.ItemConflict
	}
	if e != nil {
		return v, e
	}
	return masterDTO.ItemFromRow(row)
}
func (r *itemRepository) Event(ctx context.Context, v masterDomain.ItemEvent) error {
	before, e := masterDTO.MarshalMasterJSON(v.Before)
	if e != nil {
		return e
	}
	after, e := masterDTO.MarshalMasterJSON(v.After)
	if e != nil {
		return e
	}
	if e = r.q.ItemsRecordItemEvent(ctx, db.ItemsRecordItemEventParams{ID: masterDTO.UUID(v.ID), SubjectID: masterDTO.UUID(v.SubjectID), Action: v.Action, Reason: v.Reason, BeforeSnapshot: before, AfterSnapshot: after, ActorID: masterDTO.UUID(v.Actor)}); e != nil {
		return e
	}
	return r.j.Record(ctx, events.Event{ID: v.ID, Module: "catalogue_items", EntityType: "item", EntityID: v.SubjectID, EntityVersion: int64(v.After.Version), Action: v.Action, ActorID: v.Actor, ActorKind: "user", CorrelationID: v.ID, ScopeKind: "platform", Visibility: "restricted", OccurredAt: v.After.UpdatedAt}, nil)
}
func (r *itemRepository) History(ctx context.Context, id string) ([]masterDomain.ItemEvent, error) {
	rows, e := r.q.ItemsItemHistory(ctx, masterDTO.UUID(id))
	if e != nil {
		return nil, e
	}
	out := []masterDomain.ItemEvent{}
	for _, v := range rows {
		x := masterDomain.ItemEvent{ID: v.ID.String(), SubjectID: v.SubjectID.String(), Action: v.Action, Reason: v.Reason, Actor: v.CreatedByID.String(), CreatedAt: v.CreatedAt.Time.UTC()}
		if e = masterDTO.UnmarshalMasterJSON(v.BeforeSnapshot, &x.Before); e != nil {
			return nil, e
		}
		if e = masterDTO.UnmarshalMasterJSON(v.AfterSnapshot, &x.After); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, nil
}

type ItemValues struct{}

func (ItemValues) NormalizeModel(s string) string {
	return strings.Join(strings.FieldsFunc(cases.Fold().String(norm.NFKC.String(s)), func(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }), " ")
}

func (ItemValues) ID() string {
	return newRecordID()
}

func itemHash(v any) string {

	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(masterDTO.TransportValue(v)); err != nil {
		panic(err)
	}
	var encoded strings.Builder
	for _, r := range strings.TrimSuffix(b.String(), "\n") {
		if r < 127 {
			encoded.WriteRune(r)
		} else {
			for _, unit := range utf16.Encode([]rune{r}) {
				fmt.Fprintf(&encoded, `\u%04x`, unit)
			}
		}
	}
	raw := []byte(encoded.String())
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func (ItemValues) Keys(v masterDomain.ItemIdentity) (string, string) {
	base := map[string]any{"family": v.Family, "uom": v.UOM, "specifications": v.Specifications}
	engineering := itemHash(base)
	base["manufacturer"] = v.Manufacturer
	base["model"] = v.Model
	return engineering, itemHash(base)
}
