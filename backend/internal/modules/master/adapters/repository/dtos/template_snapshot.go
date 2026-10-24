package dtos

import (
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type TemplateDTO struct {
	ModelRequired bool               `json:"model_required"`
	ID            string             `json:"id"`
	SubcategoryID string             `json:"subcategory_id"`
	Name          string             `json:"template_name"`
	Revision      int32              `json:"revision"`
	Active        bool               `json:"is_active"`
	Fields        []TemplateFieldDTO `json:"fields"`
}

func TemplateDTOFromDomain(v domain.Template) TemplateDTO {
	return TemplateDTO{
		ModelRequired: v.ModelRequired,
		ID:            v.ID,
		SubcategoryID: v.SubcategoryID,
		Name:          v.Name,
		Revision:      v.Revision,
		Active:        v.Active,
		Fields:        dtoSlice(v.Fields, TemplateFieldDTOFromDomain),
	}
}
func (v TemplateDTO) Domain() domain.Template {
	return domain.Template{
		ModelRequired: v.ModelRequired,
		ID:            v.ID,
		SubcategoryID: v.SubcategoryID,
		Name:          v.Name,
		Revision:      v.Revision,
		Active:        v.Active,
		Fields:        dtoSlice(v.Fields, TemplateFieldDTO.Domain),
	}
}

type TemplateDefinitionDTO struct {
	ModelRequired   bool               `json:"model_required"`
	SubcategoryCode string             `json:"subcategory_code"`
	Name            string             `json:"template_name"`
	Fields          []TemplateFieldDTO `json:"fields"`
}

func TemplateDefinitionDTOFromDomain(v domain.TemplateDefinition) TemplateDefinitionDTO {
	return TemplateDefinitionDTO{
		ModelRequired:   v.ModelRequired,
		SubcategoryCode: v.SubcategoryCode,
		Name:            v.Name,
		Fields:          dtoSlice(v.Fields, TemplateFieldDTOFromDomain),
	}
}
func (v TemplateDefinitionDTO) Domain() domain.TemplateDefinition {
	return domain.TemplateDefinition{
		ModelRequired:   v.ModelRequired,
		SubcategoryCode: v.SubcategoryCode,
		Name:            v.Name,
		Fields:          dtoSlice(v.Fields, TemplateFieldDTO.Domain),
	}
}

type TemplateFieldDTO struct {
	ID               string              `json:"id,omitempty"`
	Code             string              `json:"field_code"`
	Name             string              `json:"field_name"`
	InputType        string              `json:"input_type"`
	Mandatory        bool                `json:"mandatory"`
	AllowNA          bool                `json:"allow_na"`
	UseDescription   bool                `json:"use_in_description"`
	DescriptionOrder int32               `json:"description_order"`
	Prefix           string              `json:"description_prefix"`
	Suffix           string              `json:"description_suffix"`
	UseDuplicate     bool                `json:"use_in_duplicate_check"`
	SortOrder        int32               `json:"sort_order"`
	Active           bool                `json:"is_active"`
	Options          []TemplateOptionDTO `json:"options"`
}

func TemplateFieldDTOFromDomain(v domain.TemplateField) TemplateFieldDTO {
	return TemplateFieldDTO{
		ID:               v.ID,
		Code:             v.Code,
		Name:             v.Name,
		InputType:        v.InputType,
		Mandatory:        v.Mandatory,
		AllowNA:          v.AllowNA,
		UseDescription:   v.UseDescription,
		DescriptionOrder: v.DescriptionOrder,
		Prefix:           v.Prefix,
		Suffix:           v.Suffix,
		UseDuplicate:     v.UseDuplicate,
		SortOrder:        v.SortOrder,
		Active:           v.Active,
		Options:          dtoSlice(v.Options, TemplateOptionDTOFromDomain),
	}
}
func (v TemplateFieldDTO) Domain() domain.TemplateField {
	return domain.TemplateField{
		ID:               v.ID,
		Code:             v.Code,
		Name:             v.Name,
		InputType:        v.InputType,
		Mandatory:        v.Mandatory,
		AllowNA:          v.AllowNA,
		UseDescription:   v.UseDescription,
		DescriptionOrder: v.DescriptionOrder,
		Prefix:           v.Prefix,
		Suffix:           v.Suffix,
		UseDuplicate:     v.UseDuplicate,
		SortOrder:        v.SortOrder,
		Active:           v.Active,
		Options:          dtoSlice(v.Options, TemplateOptionDTO.Domain),
	}
}

type TemplateOptionDTO struct {
	ID        string `json:"id,omitempty"`
	Code      string `json:"code"`
	Label     string `json:"label"`
	Active    bool   `json:"is_active"`
	SortOrder int32  `json:"sort_order"`
}

func TemplateOptionDTOFromDomain(v domain.TemplateOption) TemplateOptionDTO {
	return TemplateOptionDTO{
		ID:        v.ID,
		Code:      v.Code,
		Label:     v.Label,
		Active:    v.Active,
		SortOrder: v.SortOrder,
	}
}
func (v TemplateOptionDTO) Domain() domain.TemplateOption {
	return domain.TemplateOption{
		ID:        v.ID,
		Code:      v.Code,
		Label:     v.Label,
		Active:    v.Active,
		SortOrder: v.SortOrder,
	}
}
