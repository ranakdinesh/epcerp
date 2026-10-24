package dtos

import (
	"time"

	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type CategoryDTO struct {
	ID            string                    `json:"id"`
	Code          string                    `json:"code"`
	Name          string                    `json:"name"`
	Active        bool                      `json:"is_active"`
	Version       int32                     `json:"version"`
	CreatedAt     time.Time                 `json:"created_at"`
	UpdatedAt     time.Time                 `json:"updated_at"`
	CreatedBy     *string                   `json:"created_by_id"`
	UpdatedBy     *string                   `json:"updated_by_id"`
	CategoryID    string                    `json:"category_id,omitempty"`
	UnitID        string                    `json:"unit_id,omitempty"`
	Configuration *CategoryConfigurationDTO `json:"configuration,omitempty"`
}

func CategoryDTOFromDomain(v domain.Category) CategoryDTO {
	return CategoryDTO{
		ID:            v.ID,
		Code:          v.Code,
		Name:          v.Name,
		Active:        v.Active,
		Version:       v.Version,
		CreatedAt:     v.CreatedAt,
		UpdatedAt:     v.UpdatedAt,
		CreatedBy:     v.CreatedBy,
		UpdatedBy:     v.UpdatedBy,
		CategoryID:    v.CategoryID,
		UnitID:        v.UnitID,
		Configuration: dtoPointer(v.Configuration, CategoryConfigurationDTOFromDomain),
	}
}
func (v CategoryDTO) Domain() domain.Category {
	return domain.Category{
		ID:            v.ID,
		Code:          v.Code,
		Name:          v.Name,
		Active:        v.Active,
		Version:       v.Version,
		CreatedAt:     v.CreatedAt,
		UpdatedAt:     v.UpdatedAt,
		CreatedBy:     v.CreatedBy,
		UpdatedBy:     v.UpdatedBy,
		CategoryID:    v.CategoryID,
		UnitID:        v.UnitID,
		Configuration: dtoPointer(v.Configuration, CategoryConfigurationDTO.Domain),
	}
}

type CategoryCatalogueDTO struct {
	Categories    []CategoryDTO `json:"categories"`
	Subcategories []CategoryDTO `json:"subcategories"`
	Units         []CategoryDTO `json:"units"`
}

func CategoryCatalogueDTOFromDomain(v domain.CategoryCatalogue) CategoryCatalogueDTO {
	return CategoryCatalogueDTO{
		Categories:    dtoSlice(v.Categories, CategoryDTOFromDomain),
		Subcategories: dtoSlice(v.Subcategories, CategoryDTOFromDomain),
		Units:         dtoSlice(v.Units, CategoryDTOFromDomain),
	}
}
func (v CategoryCatalogueDTO) Domain() domain.CategoryCatalogue {
	return domain.CategoryCatalogue{
		Categories:    dtoSlice(v.Categories, CategoryDTO.Domain),
		Subcategories: dtoSlice(v.Subcategories, CategoryDTO.Domain),
		Units:         dtoSlice(v.Units, CategoryDTO.Domain),
	}
}

type CategoryConfigurationDTO struct {
	Fields        []CategoryFieldDTO `json:"fields"`
	ModelRequired bool               `json:"model_required"`
	FamilyCode    string             `json:"family_code"`
	Revision      int                `json:"revision"`
}

func CategoryConfigurationDTOFromDomain(v domain.CategoryConfiguration) CategoryConfigurationDTO {
	return CategoryConfigurationDTO{
		Fields:        dtoSlice(v.Fields, CategoryFieldDTOFromDomain),
		ModelRequired: v.ModelRequired,
		FamilyCode:    v.FamilyCode,
		Revision:      v.Revision,
	}
}
func (v CategoryConfigurationDTO) Domain() domain.CategoryConfiguration {
	return domain.CategoryConfiguration{
		Fields:        dtoSlice(v.Fields, CategoryFieldDTO.Domain),
		ModelRequired: v.ModelRequired,
		FamilyCode:    v.FamilyCode,
		Revision:      v.Revision,
	}
}

type CategoryCreateDTO struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	UnitID     string `json:"unit_id,omitempty"`
	CategoryID string `json:"category_id,omitempty"`
	Reason     string `json:"reason"`
}

func CategoryCreateDTOFromDomain(v domain.CategoryCreate) CategoryCreateDTO {
	return CategoryCreateDTO{
		Code:       v.Code,
		Name:       v.Name,
		UnitID:     v.UnitID,
		CategoryID: v.CategoryID,
		Reason:     v.Reason,
	}
}
func (v CategoryCreateDTO) Domain() domain.CategoryCreate {
	return domain.CategoryCreate{
		Code:       v.Code,
		Name:       v.Name,
		UnitID:     v.UnitID,
		CategoryID: v.CategoryID,
		Reason:     v.Reason,
	}
}

type CategoryEditDTO struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Reason  string `json:"reason"`
	Version int32  `json:"version"`
}

func CategoryEditDTOFromDomain(v domain.CategoryEdit) CategoryEditDTO {
	return CategoryEditDTO{
		Code:    v.Code,
		Name:    v.Name,
		Reason:  v.Reason,
		Version: v.Version,
	}
}
func (v CategoryEditDTO) Domain() domain.CategoryEdit {
	return domain.CategoryEdit{
		Code:    v.Code,
		Name:    v.Name,
		Reason:  v.Reason,
		Version: v.Version,
	}
}

type CategoryFieldDTO struct {
	Code     string              `json:"code"`
	Label    string              `json:"label"`
	Required bool                `json:"required"`
	Options  []CategoryOptionDTO `json:"options"`
}

func CategoryFieldDTOFromDomain(v domain.CategoryField) CategoryFieldDTO {
	return CategoryFieldDTO{
		Code:     v.Code,
		Label:    v.Label,
		Required: v.Required,
		Options:  dtoSlice(v.Options, CategoryOptionDTOFromDomain),
	}
}
func (v CategoryFieldDTO) Domain() domain.CategoryField {
	return domain.CategoryField{
		Code:     v.Code,
		Label:    v.Label,
		Required: v.Required,
		Options:  dtoSlice(v.Options, CategoryOptionDTO.Domain),
	}
}

type CategoryOptionDTO struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

func CategoryOptionDTOFromDomain(v domain.CategoryOption) CategoryOptionDTO {
	return CategoryOptionDTO{
		Code:  v.Code,
		Label: v.Label,
	}
}
func (v CategoryOptionDTO) Domain() domain.CategoryOption {
	return domain.CategoryOption{
		Code:  v.Code,
		Label: v.Label,
	}
}

type CategoryPageDTO struct {
	Rows  []CategorySummaryDTO `json:"rows"`
	Total int64                `json:"total"`
	Page  int32                `json:"page"`
	Size  int32                `json:"size"`
}

func CategoryPageDTOFromDomain(v domain.CategoryPage) CategoryPageDTO {
	return CategoryPageDTO{
		Rows:  dtoSlice(v.Rows, CategorySummaryDTOFromDomain),
		Total: v.Total,
		Page:  v.Page,
		Size:  v.Size,
	}
}
func (v CategoryPageDTO) Domain() domain.CategoryPage {
	return domain.CategoryPage{
		Rows:  dtoSlice(v.Rows, CategorySummaryDTO.Domain),
		Total: v.Total,
		Page:  v.Page,
		Size:  v.Size,
	}
}

type CategorySourceDTO struct {
	ID        string                `json:"id"`
	Code      string                `json:"code"`
	Name      string                `json:"name"`
	Data      CategorySourceDataDTO `json:"data"`
	Active    bool                  `json:"is_active"`
	Version   int                   `json:"version"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
}

func CategorySourceDTOFromDomain(v domain.CategorySource) CategorySourceDTO {
	return CategorySourceDTO{
		ID:        v.ID,
		Code:      v.Code,
		Name:      v.Name,
		Data:      CategorySourceDataDTOFromDomain(v.Data),
		Active:    v.Active,
		Version:   v.Version,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}
func (v CategorySourceDTO) Domain() domain.CategorySource {
	return domain.CategorySource{
		ID:        v.ID,
		Code:      v.Code,
		Name:      v.Name,
		Data:      v.Data.Domain(),
		Active:    v.Active,
		Version:   v.Version,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

type CategorySourceDataDTO struct {
	Kind               string             `json:"kind"`
	CategoryCode       string             `json:"category_code,omitempty"`
	UOM                string             `json:"uom,omitempty"`
	Fields             []CategoryFieldDTO `json:"fields,omitempty"`
	ModelRequired      bool               `json:"model_required,omitempty"`
	FamilyCode         string             `json:"family_code,omitempty"`
	Revision           int                `json:"revision,omitempty"`
	Source             string             `json:"source,omitempty"`
	LegacyCategoryCode string             `json:"legacy_category_code,omitempty"`
}

func CategorySourceDataDTOFromDomain(v domain.CategorySourceData) CategorySourceDataDTO {
	return CategorySourceDataDTO{
		Kind:               v.Kind,
		CategoryCode:       v.CategoryCode,
		UOM:                v.UOM,
		Fields:             dtoSlice(v.Fields, CategoryFieldDTOFromDomain),
		ModelRequired:      v.ModelRequired,
		FamilyCode:         v.FamilyCode,
		Revision:           v.Revision,
		Source:             v.Source,
		LegacyCategoryCode: v.LegacyCategoryCode,
	}
}
func (v CategorySourceDataDTO) Domain() domain.CategorySourceData {
	return domain.CategorySourceData{
		Kind:               v.Kind,
		CategoryCode:       v.CategoryCode,
		UOM:                v.UOM,
		Fields:             dtoSlice(v.Fields, CategoryFieldDTO.Domain),
		ModelRequired:      v.ModelRequired,
		FamilyCode:         v.FamilyCode,
		Revision:           v.Revision,
		Source:             v.Source,
		LegacyCategoryCode: v.LegacyCategoryCode,
	}
}

type CategorySummaryDTO struct {
	ID            string                    `json:"id"`
	Code          string                    `json:"code"`
	Name          string                    `json:"name"`
	Active        bool                      `json:"is_active"`
	Version       int32                     `json:"version"`
	CategoryID    string                    `json:"category_id,omitempty"`
	UnitID        string                    `json:"unit_id,omitempty"`
	Configuration *CategoryConfigurationDTO `json:"configuration,omitempty"`
	UnitCode      string                    `json:"unit_code"`
	UnitName      string                    `json:"unit_name"`
	ChildCount    int64                     `json:"child_count"`
}

func CategorySummaryDTOFromDomain(v domain.CategorySummary) CategorySummaryDTO {
	return CategorySummaryDTO{
		ID:            v.ID,
		Code:          v.Code,
		Name:          v.Name,
		Active:        v.Active,
		Version:       v.Version,
		CategoryID:    v.CategoryID,
		UnitID:        v.UnitID,
		Configuration: dtoPointer(v.Configuration, CategoryConfigurationDTOFromDomain),
		UnitCode:      v.UnitCode,
		UnitName:      v.UnitName,
		ChildCount:    v.ChildCount,
	}
}
func (v CategorySummaryDTO) Domain() domain.CategorySummary {
	return domain.CategorySummary{
		ID:            v.ID,
		Code:          v.Code,
		Name:          v.Name,
		Active:        v.Active,
		Version:       v.Version,
		CategoryID:    v.CategoryID,
		UnitID:        v.UnitID,
		Configuration: dtoPointer(v.Configuration, CategoryConfigurationDTO.Domain),
		UnitCode:      v.UnitCode,
		UnitName:      v.UnitName,
		ChildCount:    v.ChildCount,
	}
}
