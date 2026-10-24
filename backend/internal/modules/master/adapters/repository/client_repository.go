package repository

import (
	"context"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/helpers"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type ClientUnitOfWork struct {
	manager   *database.Manager
	directory func(database.Connection) masterContracts.ReferenceReader
}

func NewClientUnitOfWork(b database.Beginner, d func(database.Connection) masterContracts.ReferenceReader) *ClientUnitOfWork {
	return &ClientUnitOfWork{database.NewManager(b), d}
}
func (u *ClientUnitOfWork) Within(ctx context.Context, work func(context.Context, masterPorts.ClientRepository, masterPorts.ClientDirectory) error) error {
	return u.manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return work(ctx, &clientRepository{NewStore(c).Queries}, &clientDirectoryBridge{u.directory(c)})
	})
}

type clientDirectoryBridge struct {
	reader masterContracts.ReferenceReader
}

func (b *clientDirectoryBridge) Get(ctx context.Context, k, id string) (*masterDomain.ClientReference, error) {
	r, e := b.reader.Get(ctx, k, id)
	if e != nil || r == nil {
		return nil, e
	}
	return &masterDomain.ClientReference{ID: r.ID, Name: r.Name, Active: r.Active, StateID: r.StateID}, nil
}

type ClientSystemValues struct{}

func (ClientSystemValues) ID() string {
	return newRecordID()
}

type clientRepository struct{ q *db.Queries }

func (r *clientRepository) loadContacts(ctx context.Context, c *masterDomain.Client) error {
	rows, e := r.q.ClientsClientContacts(ctx, masterDTO.UUID(c.ID))
	if e != nil {
		return e
	}
	for _, v := range rows {
		c.Contacts = append(c.Contacts, masterDomain.ClientContact{ID: v.ID.String(), FullName: v.FullName, Type: v.ContactType, Designation: v.Designation, Department: v.Department, Email: v.Email, Phone: v.Phone, Primary: v.IsPrimary, Active: v.IsActive})
	}
	return nil
}
func (r *clientRepository) Get(ctx context.Context, id string, lock bool) (*masterDomain.Client, error) {
	var row db.ClientsGetClientRow
	var e error
	if lock {
		var locked db.ClientsLockClientRow
		locked, e = r.q.ClientsLockClient(ctx, masterDTO.UUID(id))
		row = db.ClientsGetClientRow(locked)
	} else {
		row, e = r.q.ClientsGetClient(ctx, masterDTO.UUID(id))
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	c := masterDTO.ClientFromRow(row)
	if e = r.loadContacts(ctx, &c); e != nil {
		return nil, e
	}
	return &c, nil
}
func (r *clientRepository) Save(ctx context.Context, c masterDomain.Client, create bool) error {
	var e error
	if create {
		e = r.q.ClientsInsertClient(ctx, db.ClientsInsertClientParams{ID: masterDTO.UUID(c.ID), ClientCode: c.Code, NormalizedLegalName: c.NormalizedName, IsActive: c.Active, CreatedByID: masterDTO.UUID(*c.CreatedBy), UpdatedByID: masterDTO.UUID(*c.UpdatedBy),
			LegalName: c.LegalName, DisplayName: c.DisplayName, ParentOrganization: c.ParentOrganization, ClientType: c.ClientType, Industry: c.Industry, Gstin: c.Gstin, Pan: c.Pan, Cin: c.Cin, Website: c.Website, AddressLine1: c.AddressLine1, AddressLine2: c.AddressLine2, City: c.City, State: c.State, PostalCode: c.PostalCode, Country: c.Country})
	} else {
		e = r.q.ClientsUpdateClient(ctx, db.ClientsUpdateClientParams{ID: masterDTO.UUID(c.ID), NormalizedLegalName: c.NormalizedName, IsActive: c.Active, UpdatedByID: masterDTO.UUID(*c.UpdatedBy),
			LegalName: c.LegalName, DisplayName: c.DisplayName, ParentOrganization: c.ParentOrganization, ClientType: c.ClientType, Industry: c.Industry, Gstin: c.Gstin, Pan: c.Pan, Cin: c.Cin, Website: c.Website, AddressLine1: c.AddressLine1, AddressLine2: c.AddressLine2, City: c.City, State: c.State, PostalCode: c.PostalCode, Country: c.Country})
	}
	var pg *pgconn.PgError
	if errors.As(e, &pg) && pg.Code == "23505" {
		return masterDomain.ClientDuplicate
	}
	return e
}
func (r *clientRepository) Contacts(ctx context.Context, id string, contacts []masterDomain.ClientContact, actorID string, retire bool) error {
	existing, e := r.q.ClientsClientContacts(ctx, masterDTO.UUID(id))
	if e != nil {
		return e
	}
	known := map[string]bool{}
	for _, v := range existing {
		known[v.ID.String()] = true
	}
	ids := []pgtype.UUID{}
	for _, v := range contacts {
		ids = append(ids, masterDTO.UUID(v.ID))
	}
	if e = r.q.ClientsRetireContacts(ctx, db.ClientsRetireContactsParams{ClientID: masterDTO.UUID(id), ActorID: masterDTO.UUID(actorID), RetainedIds: ids}); e != nil {
		return e
	}
	if retire {
		return nil
	}
	for _, v := range contacts {
		if known[v.ID] {
			e = r.q.ClientsUpdateContact(ctx, db.ClientsUpdateContactParams{ID: masterDTO.UUID(v.ID), ClientID: masterDTO.UUID(id), FullName: v.FullName, ContactType: v.Type, Designation: v.Designation, Department: v.Department, Email: v.Email, Phone: v.Phone, IsPrimary: v.Primary, IsActive: v.Active, UpdatedByID: masterDTO.UUID(actorID)})
		} else {
			e = r.q.ClientsInsertContact(ctx, db.ClientsInsertContactParams{ID: masterDTO.UUID(v.ID), ClientID: masterDTO.UUID(id), FullName: v.FullName, ContactType: v.Type, Designation: v.Designation, Department: v.Department, Email: v.Email, Phone: v.Phone, IsPrimary: v.Primary, IsActive: v.Active, CreatedByID: masterDTO.UUID(actorID), UpdatedByID: masterDTO.UUID(actorID)})
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func (r *clientRepository) List(ctx context.Context, f masterDomain.ClientFilter) (masterDomain.ClientPage, error) {
	p := masterDomain.ClientPage{Items: []masterDomain.Client{}, Page: f.Page, Size: f.Size}
	search := ""
	if f.Search != "" {
		search = "%" + helpers.Fold(f.Search) + "%"
	}
	var active pgtype.Bool
	if f.Active != nil {
		active = pgtype.Bool{Bool: *f.Active, Valid: true}
	}
	rows, e := r.q.ClientsListClients(ctx, db.ClientsListClientsParams{Search: search, Active: active, ClientType: helpers.Fold(f.ClientType), Sort: f.Sort, Descending: f.Descending, PageSize: f.Size, PageOffset: (f.Page - 1) * f.Size})
	if e != nil {
		return p, e
	}
	for _, v := range rows {
		c := masterDTO.ClientFromRow(db.ClientsGetClientRow(v))
		if e = r.loadContacts(ctx, &c); e != nil {
			return p, e
		}
		p.Items = append(p.Items, c)
	}
	p.Total, e = r.q.ClientsCountClients(ctx, db.ClientsCountClientsParams{Search: search, Active: active, ClientType: helpers.Fold(f.ClientType)})
	if e != nil {
		return p, e
	}
	summary, e := r.q.ClientsSummary(ctx)
	p.All = summary.AllCount
	p.Active = summary.ActiveCount
	p.Inactive = summary.InactiveCount
	p.TotalPages = (p.Total + int64(f.Size) - 1) / int64(f.Size)
	return p, e
}
