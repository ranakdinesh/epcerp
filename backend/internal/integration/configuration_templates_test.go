package integration

import (
	"bytes"
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	authInfrastructure "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/schema"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventout "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventapp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"

	masterHTTP "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	"net/http/httptest"
	"os"
	"testing"
)

func approvedTemplates(t *testing.T) []masterDomain.TemplateDefinition {
	t.Helper()
	raw, e := os.ReadFile("testdata/configuration_templates.json")
	if e != nil {
		t.Fatal(e)
	}
	var source struct {
		Templates []struct {
			ModelRequired   bool   `json:"model_required"`
			SubcategoryCode string `json:"subcategory_code"`
			Name            string `json:"template_name"`
			Fields          []struct {
				masterDTO.TemplateFieldDTO
				Choices []struct {
					Code  string `json:"code"`
					Label string `json:"label"`
				} `json:"proposed_options_from_python"`
			}
		} `json:"templates"`
	}
	if e = masterDTO.UnmarshalJSON(raw, &source); e != nil {
		t.Fatal(e)
	}
	defs := []masterDomain.TemplateDefinition{}
	for _, v := range source.Templates {
		d := masterDomain.TemplateDefinition{SubcategoryCode: v.SubcategoryCode, Name: v.Name, ModelRequired: v.ModelRequired}
		for _, f := range v.Fields {
			field := f.TemplateFieldDTO.Domain()
			for i, o := range f.Choices {
				field.Options = append(field.Options, masterDomain.TemplateOption{Code: o.Code, Label: o.Label, Active: true, SortOrder: int32(i + 1)})
			}
			d.Fields = append(d.Fields, field)
		}
		defs = append(defs, d)
	}
	return defs
}
func TestConfigurationTemplateAdoptionAndPreview(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	actor := "135bcaf6-e4ec-4809-b7e9-42b94c332891"
	execFixture(t, pool, `INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,'template_fixture','template@example.invalid','fixture',true,false)`, actor)
	execFixture(t, pool, `INSERT INTO catalogue_units(id,code,name)VALUES(md5('tu')::uuid,'M','Meter')`)
	execFixture(t, pool, `INSERT INTO categories(id,code,name,unit_id)VALUES(md5('tc')::uuid,'CAT','Catalogue',md5('tu')::uuid)`)
	defs := approvedTemplates(t)
	for _, d := range defs {
		execFixture(t, pool, `INSERT INTO category_subcategories(id,code,name,category_id,unit_id,configuration)VALUES(md5($1)::uuid,$1,$1,md5('tc')::uuid,md5('tu')::uuid,'{"fields":[]}'::jsonb)`, d.SubcategoryCode)
	}
	identities := projectIdentities{"admin": auth.Identity{ID: actor, Active: true, Superuser: true}, "reader": auth.Identity{ID: actor, Active: true, Permissions: []string{"settings.configuration_templates.read"}}, "writer": auth.Identity{ID: actor, Active: true, Permissions: []string{"settings.configuration_templates.write"}}, "denied": auth.Identity{ID: actor, Active: true}}
	journal := func(c database.Connection) events.Writer { return eventapp.NewRecorder(eventout.NewJournal(c)) }
	newService := func(j func(database.Connection) events.Writer) *masterApplication.TemplateService {
		return masterApplication.NewTemplateService(masterRepository.NewTemplateUnitOfWork(pool, func(c database.Connection) masterContracts.CategoryDirectory {
			return masterRepository.NewCategoryDirectory(c)
		}, j), authInfrastructure.NewAuthBridge(identities), masterRepository.TemplateValues{})
	}
	service := newService(journal)
	if _, e := service.Import(ctx, "reader", defs); !errors.Is(e, masterDomain.TemplateForbidden) {
		t.Fatal("reader imported", e)
	}
	if _, e := newService(func(database.Connection) events.Writer { return failingCategoryJournal{} }).Import(ctx, "admin", defs); e == nil {
		t.Fatal("audit failure committed")
	}
	var count int
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM configuration_templates`).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 0 {
		t.Fatal("audit rollback failed")
	}
	result, e := service.Import(ctx, "admin", defs)
	if e != nil || result.Created != 10 {
		t.Fatal(result, e)
	}
	result, e = service.Import(ctx, "admin", defs)
	if e != nil || result.Unchanged != 10 || result.Created != 0 {
		t.Fatal("repeat not idempotent", result, e)
	}
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM template_fields`).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 51 {
		t.Fatal("field count", count)
	}
	expectedOptions := 0
	for _, d := range defs {
		for _, f := range d.Fields {
			expectedOptions += len(f.Options)
		}
	}
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM template_options`).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != expectedOptions {
		t.Fatal("option count", count, expectedOptions)
	}
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM business_events WHERE source_module='configuration_templates'`).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 10 {
		t.Fatal("event count", count)
	}
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM template_fields WHERE created_by_id=$1 AND updated_by_id=$1`, actor).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 51 {
		t.Fatal("field actors")
	}
	for _, d := range defs {
		var id string
		if e := pool.QueryRow(ctx, `SELECT id::text FROM category_subcategories WHERE code=$1`, d.SubcategoryCode).Scan(&id); e != nil {
			t.Fatal(e)
		}
		v, e := service.Get(ctx, "reader", id)
		if e != nil || v.Name != d.Name || v.ModelRequired != d.ModelRequired || len(v.Fields) != len(d.Fields) {
			t.Fatal("read adoption", e)
		}
		for i, f := range v.Fields {
			source := d.Fields[i]
			if f.Code != source.Code || f.Name != source.Name || f.AllowNA != source.AllowNA || f.Mandatory != source.Mandatory || f.UseDescription != source.UseDescription || f.DescriptionOrder != source.DescriptionOrder || f.Prefix != source.Prefix || f.Suffix != source.Suffix || f.UseDuplicate != source.UseDuplicate || f.SortOrder != source.SortOrder || len(f.Options) != len(source.Options) {
				t.Fatal("metadata mismatch", d.Name, f.Code)
			}
			for j, o := range f.Options {
				if o.Code != source.Options[j].Code || o.Label != source.Options[j].Label || !o.Active {
					t.Fatal("option mismatch")
				}
			}
		}
	}
	var id string
	if e := pool.QueryRow(ctx, `SELECT id::text FROM category_subcategories WHERE code='LTPC'`).Scan(&id); e != nil {
		t.Fatal(e)
	}
	v, e := service.Get(ctx, "reader", id)
	if e != nil {
		t.Fatal(e)
	}
	values := map[string]string{}
	for _, f := range v.Fields {
		if f.Mandatory {
			values[f.Code] = f.Options[0].Code
		}
	}
	values["ARM"] = masterDomain.TemplateNA
	p, e := service.Preview(ctx, "reader", id, values)
	if e != nil || p.Description == "" {
		t.Fatal("preview", e)
	}
	values["SH"] = v.Fields[6].Options[0].Code
	p2, e := service.Preview(ctx, "reader", id, values)
	if e != nil || p.EngineeringKey != p2.EngineeringKey || p.Description == p2.Description {
		t.Fatal("duplicate flags ignored", e)
	}
	if _, e = service.Get(ctx, "denied", id); !errors.Is(e, masterDomain.TemplateForbidden) {
		t.Fatal("read permission", e)
	}
	defs[0].Name = "Changed"
	if _, e = service.Import(ctx, "admin", defs); !errors.Is(e, masterDomain.TemplateConflict) {
		t.Fatal("conflict overwritten", e)
	}
	if _, e = pool.Exec(ctx, `UPDATE template_options SET option_label='tampered'`); e == nil {
		t.Fatal("immutable options changed")
	}
	router := chi.NewRouter()
	masterHTTP.RegisterTemplateRoutes(router, service)
	for _, body := range []string{`{"values":{},"extra":true}`, `{} {}`, `{"values":{"C":"bad"}}`} {
		req := httptest.NewRequest("POST", "/api/v1/configuration-templates/"+id+"/preview", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer reader")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 422 {
			t.Fatal("strict preview", w.Code)
		}
	}

	for _, body := range []string{`{}`, `{"expected_revision":0,"extra":true}`, `{} {}`} {
		req := httptest.NewRequest("POST", "/api/v1/configuration-templates/"+id+"/publish", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer admin")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 422 {
			t.Fatal("strict publish boundary", w.Code)
		}
	}
	// Native edits publish immutable revisions with atomic audit and event evidence.
	def := approvedTemplates(t)[0]
	def.Fields[0].Name = "Core count"
	def.Fields[0].Options = append(def.Fields[0].Options, masterDomain.TemplateOption{Code: "99C", Label: "99 Core", Active: true, SortOrder: 99})
	def.Fields[0].Options[0].Active = false
	if _, e = service.Publish(ctx, "reader", id, def, 1, "Reviewed correction"); !errors.Is(e, masterDomain.TemplateForbidden) {
		t.Fatal("read-only user published", e)
	}
	if _, e = service.Publish(ctx, "admin", id, def, 1, " "); e == nil {
		t.Fatal("missing review reason accepted")
	}
	if _, e = newService(func(database.Connection) events.Writer { return failingCategoryJournal{} }).Publish(ctx, "admin", id, def, 1, "Reviewed correction"); e == nil {
		t.Fatal("failed publication audit committed")
	}
	current, e := service.Get(ctx, "reader", id)
	if e != nil || current.Revision != 1 {
		t.Fatal("publication rollback", e)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM template_publications`).Scan(&count); e != nil || count != 0 {
		t.Fatal("publication history rollback", e)
	}
	published, e := service.Publish(ctx, "writer", id, def, 1, "Reviewed correction")
	if e != nil || published.Revision != 2 {
		t.Fatal("native revision", e)
	}
	current, e = service.Get(ctx, "reader", id)
	if e != nil || current.Fields[0].Name != "Core count" || current.Fields[0].Options[0].Active {
		t.Fatal("latest edit not returned", e)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM template_fields WHERE template_id=$1 AND field_name='Number of Cores'`, v.ID).Scan(&count); e != nil || count != 1 {
		t.Fatal("old revision changed", e)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM template_publications WHERE template_id=$1 AND previous_template_id=$2 AND reason='Reviewed correction' AND created_by_id=$3 AND updated_by_id=$3 AND before_definition->>'revision'='1' AND after_definition->>'template_name'=$4`, published.ID, v.ID, actor, def.Name).Scan(&count); e != nil || count != 1 {
		t.Fatal("publication audit evidence", e)
	}
	if _, e = service.Publish(ctx, "admin", id, def, 1, "stale editor"); e == nil {
		t.Fatal("stale publication accepted")
	}
	removed := def
	removed.Fields = append([]masterDomain.TemplateField(nil), def.Fields[1:]...)
	if _, e = service.Publish(ctx, "admin", id, removed, 2, "remove identity"); e == nil {
		t.Fatal("removed stable identity accepted")
	}
	if _, e = pool.Exec(ctx, `UPDATE template_publications SET reason='tampered'`); e == nil {
		t.Fatal("audit history mutable")
	}
	execFixture(t, pool, `INSERT INTO category_subcategories(id,code,name,category_id,unit_id,configuration)VALUES(md5('NEW')::uuid,'NEW','New type',md5('tc')::uuid,md5('tu')::uuid,'{}'::jsonb)`)
	var newID string
	if e = pool.QueryRow(ctx, `SELECT id::text FROM category_subcategories WHERE code='NEW'`).Scan(&newID); e != nil {
		t.Fatal(e)
	}
	def.SubcategoryCode = "NEW" // Same template name is valid for another parent.
	if _, e = service.Publish(ctx, "admin", newID, def, 0, "Initial native definition"); e != nil {
		t.Fatal("new template", e)
	}
	if _, e = service.Publish(ctx, "admin", id, def, 2, "wrong parent"); e == nil {
		t.Fatal("parent mismatch accepted")
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM business_events WHERE source_module='configuration_templates' AND action='publish'`).Scan(&count); e != nil || count != 2 {
		t.Fatal("publication event count", e)
	}
	m, e := schema.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	if e = m.Migrate(17); e == nil {
		t.Fatal("populated revision rollback destroyed evidence")
	}
}
