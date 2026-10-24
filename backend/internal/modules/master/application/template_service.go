package application

import (
	"context"
	"errors"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type TemplateService struct {
	u masterPorts.TemplateUnitOfWork
	a masterPorts.Authorizer
	v masterPorts.TemplateValues
}

func NewTemplateService(u masterPorts.TemplateUnitOfWork, a masterPorts.Authorizer, v masterPorts.TemplateValues) *TemplateService {
	return &TemplateService{u, a, v}
}
func (s *TemplateService) actor(ctx context.Context, token, permission string) (sharedIdentity.Principal, error) {
	p, e := s.a.Resolve(ctx, token)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, masterDomain.TemplateUnauthorized
	}
	if !p.Allows(permission) {
		return p, masterDomain.TemplateForbidden
	}
	return p, nil
}
func (s *TemplateService) Get(ctx context.Context, token, id string) (out masterDomain.Template, e error) {
	if _, e = s.actor(ctx, token, "settings.configuration_templates.read"); e != nil {
		return
	}
	if e = masterDomain.TemplateValidateID(id); e != nil {
		return
	}
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.TemplateRepository, _ masterPorts.TemplateDirectory) error {
		out, e = r.Get(ctx, id)
		return e
	})
	return
}
func (s *TemplateService) Preview(ctx context.Context, token, id string, values map[string]string) (masterDomain.TemplatePreview, error) {
	t, e := s.Get(ctx, token, id)
	if e != nil {
		return masterDomain.TemplatePreview{}, e
	}
	preview, e := masterDomain.TemplateEvaluate(t, values)
	if e == nil {
		preview.EngineeringKey = s.v.EngineeringKey(preview.Identity)
	}
	return preview, e
}
func (s *TemplateService) Import(ctx context.Context, token string, defs []masterDomain.TemplateDefinition) (result masterDomain.TemplateResult, e error) {
	p, e := s.actor(ctx, token, "settings.configuration_templates.import")
	if e != nil {
		return result, e
	}
	if e = masterDomain.TemplateValidate(defs); e != nil {
		return result, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.TemplateRepository, d masterPorts.TemplateDirectory) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		for _, def := range defs {
			parent, e := d.Resolve(ctx, def.SubcategoryCode)
			if e != nil {
				return e
			}
			existing, e := r.Get(ctx, parent.ID)
			fingerprint := s.v.Fingerprint(def)
			if e == nil {
				if existing.Fingerprint != fingerprint {
					return masterDomain.TemplateConflict
				}
				result.Unchanged++
				continue
			}
			if !errors.Is(e, masterDomain.TemplateMissing) {
				return e
			}
			t := masterDomain.Template{ID: s.v.ID(), SubcategoryID: parent.ID, Name: def.Name, Revision: 1, ModelRequired: def.ModelRequired, Active: true, Fingerprint: fingerprint, Fields: append([]masterDomain.TemplateField(nil), def.Fields...)}
			for i := range t.Fields {
				t.Fields[i].ID = s.v.ID()
				t.Fields[i].Options = append([]masterDomain.TemplateOption(nil), def.Fields[i].Options...)
				for j := range t.Fields[i].Options {
					t.Fields[i].Options[j].ID = s.v.ID()
				}
			}
			if e = r.Insert(ctx, t, def, p.ID); e != nil {
				return e
			}
			result.Created++
		}
		return nil
	})
	if e != nil {
		return masterDomain.TemplateResult{}, e
	}
	return result, nil
}

// Publish serializes adoption and revisions; the expected revision guards stale editors.
func (s *TemplateService) Publish(ctx context.Context, token, id string, def masterDomain.TemplateDefinition, expected int32, reason string) (out masterDomain.Template, e error) {
	p, e := s.actor(ctx, token, "settings.configuration_templates.write")
	if e != nil {
		return out, e
	}
	if e = masterDomain.TemplateValidateID(id); e != nil {
		return out, e
	}
	if e = masterDomain.TemplateValidatePublication(expected, reason); e != nil {
		return out, e
	}
	if e = masterDomain.TemplateValidate([]masterDomain.TemplateDefinition{def}); e != nil {
		return out, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r masterPorts.TemplateRepository, d masterPorts.TemplateDirectory) error {
		if e := r.Lock(ctx); e != nil {
			return e
		}
		parent, e := d.Resolve(ctx, def.SubcategoryCode)
		if e != nil {
			return e
		}
		if parent.ID != id {
			return masterDomain.TemplateInvalid("Subcategory code does not match the selected parent")
		}
		before, e := r.Get(ctx, id)
		if e != nil && !errors.Is(e, masterDomain.TemplateMissing) {
			return e
		}
		if e = masterDomain.TemplateValidateRevision(before, def, expected); e != nil {
			return e
		}
		if !parent.Active {
			return masterDomain.TemplateInvalid("Activate the subcategory, category and unit before configuring it")
		}
		out = masterDomain.Template{ID: s.v.ID(), SubcategoryID: id, Name: def.Name, ModelRequired: def.ModelRequired, Revision: expected + 1, Active: true, Fingerprint: s.v.Fingerprint(def), Fields: append([]masterDomain.TemplateField(nil), def.Fields...)}
		for i := range out.Fields {
			out.Fields[i].ID = s.v.ID()
			out.Fields[i].Options = append([]masterDomain.TemplateOption(nil), def.Fields[i].Options...)
			for j := range out.Fields[i].Options {
				out.Fields[i].Options[j].ID = s.v.ID()
			}
		}
		return r.Publish(ctx, out, before, def, p.ID, reason)
	})
	return out, e
}
