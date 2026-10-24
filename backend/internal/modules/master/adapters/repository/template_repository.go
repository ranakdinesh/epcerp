package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type TemplateUnitOfWork struct {
	manager   *database.Manager
	directory func(database.Connection) masterContracts.CategoryDirectory
	journal   func(database.Connection) events.Writer
}

func NewTemplateUnitOfWork(b database.Beginner, d func(database.Connection) masterContracts.CategoryDirectory, j func(database.Connection) events.Writer) *TemplateUnitOfWork {
	return &TemplateUnitOfWork{database.NewManager(b), d, j}
}
func (u *TemplateUnitOfWork) Within(ctx context.Context, f func(context.Context, masterPorts.TemplateRepository, masterPorts.TemplateDirectory) error) error {
	return u.manager.Within(ctx, func(ctx context.Context, c database.Connection) error {
		return f(ctx, &templateRepository{NewStore(c).Queries, u.journal(c)}, templateDirectory{u.directory(c)})
	})
}

type templateDirectory struct {
	masterContracts.CategoryDirectory
}

func (d templateDirectory) Resolve(ctx context.Context, code string) (masterDomain.TemplateSubcategory, error) {
	v, found, e := d.FindSubcategory(ctx, code)
	if e != nil {
		return masterDomain.TemplateSubcategory{}, e
	}
	if !found {
		return masterDomain.TemplateSubcategory{}, masterDomain.TemplateInvalid("Subcategory does not exist")
	}
	return masterDomain.TemplateSubcategory{ID: v.ID, Active: v.Active}, nil
}

type templateRepository struct {
	q *db.Queries
	j events.Writer
}

func (r *templateRepository) Lock(ctx context.Context) error { return r.q.TemplatesLockTemplates(ctx) }
func (r *templateRepository) Get(ctx context.Context, id string) (masterDomain.Template, error) {
	v, e := r.q.TemplatesGetTemplate(ctx, masterDTO.UUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return masterDomain.Template{}, masterDomain.TemplateMissing
	}
	if e != nil {
		return masterDomain.Template{}, e
	}
	return r.template(ctx, v)
}
func (r *templateRepository) template(ctx context.Context, v db.ConfigurationTemplate) (masterDomain.Template, error) {
	var e error
	out := masterDomain.Template{ID: v.ID.String(), SubcategoryID: v.SubcategoryID.String(), Name: v.TemplateName, ModelRequired: v.ModelRequired, Revision: v.Revision, Active: v.IsActive, Fingerprint: v.SourceFingerprint, Fields: []masterDomain.TemplateField{}}
	fields, e := r.q.TemplatesListFields(ctx, v.ID)
	if e != nil {
		return out, e
	}
	options, e := r.q.TemplatesListOptions(ctx, v.ID)
	if e != nil {
		return out, e
	}
	byField := map[string][]masterDomain.TemplateOption{}
	for _, o := range options {
		key := o.FieldID.String()
		byField[key] = append(byField[key], masterDomain.TemplateOption{ID: o.ID.String(), Code: o.OptionCode, Label: o.OptionLabel, Active: o.IsActive, SortOrder: o.SortOrder})
	}
	for _, f := range fields {
		out.Fields = append(out.Fields, masterDomain.TemplateField{ID: f.ID.String(), Code: f.FieldCode, Name: f.FieldName, InputType: f.InputType, Mandatory: f.Mandatory, AllowNA: f.AllowNa, UseDescription: f.UseInDescription, DescriptionOrder: f.DescriptionOrder, Prefix: f.DescriptionPrefix, Suffix: f.DescriptionSuffix, UseDuplicate: f.UseInDuplicateCheck, SortOrder: f.SortOrder, Active: f.IsActive, Options: byField[f.ID.String()]})
	}
	return out, nil
}
func (r *templateRepository) Insert(ctx context.Context, t masterDomain.Template, source masterDomain.TemplateDefinition, actor string) error {
	return r.insert(ctx, t, source, actor, "import")
}
func (r *templateRepository) insert(ctx context.Context, t masterDomain.Template, source masterDomain.TemplateDefinition, actor, action string) error {
	raw, e := masterDTO.MarshalMasterJSON(source)
	if action == "publish" {
		raw = []byte(`{}`)
	}
	if e != nil {
		return e
	}
	if e = r.q.TemplatesInsertTemplate(ctx, db.TemplatesInsertTemplateParams{ID: masterDTO.UUID(t.ID), SubcategoryID: masterDTO.UUID(t.SubcategoryID), TemplateName: t.Name, ModelRequired: t.ModelRequired, Revision: t.Revision, IsActive: t.Active, LegacyDefinition: raw, SourceFingerprint: t.Fingerprint, ActorID: masterDTO.UUID(actor)}); e != nil {
		return e
	}
	for _, f := range t.Fields {
		if e = r.q.TemplatesInsertField(ctx, db.TemplatesInsertFieldParams{ID: masterDTO.UUID(f.ID), TemplateID: masterDTO.UUID(t.ID), FieldCode: f.Code, FieldName: f.Name, InputType: f.InputType, Mandatory: f.Mandatory, AllowNa: f.AllowNA, UseInDescription: f.UseDescription, DescriptionOrder: f.DescriptionOrder, DescriptionPrefix: f.Prefix, DescriptionSuffix: f.Suffix, UseInDuplicateCheck: f.UseDuplicate, SortOrder: f.SortOrder, IsActive: f.Active, ActorID: masterDTO.UUID(actor)}); e != nil {
			return e
		}
		for _, o := range f.Options {
			if e = r.q.TemplatesInsertOption(ctx, db.TemplatesInsertOptionParams{ID: masterDTO.UUID(o.ID), FieldID: masterDTO.UUID(f.ID), OptionCode: o.Code, OptionLabel: o.Label, SortOrder: o.SortOrder, IsActive: o.Active, ActorID: masterDTO.UUID(actor)}); e != nil {
				return e
			}
		}
	}
	id := TemplateValues{}.ID()
	return r.j.Record(ctx, events.Event{ID: id, Module: "configuration_templates", EntityType: "template", EntityID: t.ID, EntityVersion: int64(t.Revision), Action: action, ActorID: actor, ActorKind: "user", CorrelationID: id, ScopeKind: "platform", Visibility: "restricted", OccurredAt: time.Now().UTC()}, nil)
}

type TemplateValues struct{}

func (TemplateValues) ID() string {
	return newRecordID()
}

func (TemplateValues) Fingerprint(d masterDomain.TemplateDefinition) string {
	b, _ := masterDTO.MarshalMasterJSON(d)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (TemplateValues) EngineeringKey(v masterDomain.TemplateIdentity) string {
	b, _ := masterDTO.MarshalMasterJSON(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (r *templateRepository) Publish(ctx context.Context, t, before masterDomain.Template, def masterDomain.TemplateDefinition, actor, reason string) error {
	if e := r.insert(ctx, t, masterDomain.TemplateDefinition{}, actor, "publish"); e != nil {
		return e
	}
	previous, e := masterDTO.MarshalMasterJSON(before)
	if e != nil {
		return e
	}
	next, e := masterDTO.MarshalMasterJSON(def)
	if e != nil {
		return e
	}
	return r.q.TemplatesInsertPublication(ctx, db.TemplatesInsertPublicationParams{TemplateID: masterDTO.UUID(t.ID), PreviousTemplateID: masterDTO.UUID(before.ID), Reason: reason, BeforeDefinition: previous, AfterDefinition: next, ActorID: masterDTO.UUID(actor)})
}
