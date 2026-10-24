package dtos

import (
	"time"

	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ItemDTO struct {
	ID              string            `json:"id"`
	ConfigurationID string            `json:"configuration_id"`
	ManufacturerID  string            `json:"manufacturer_id"`
	Model           string            `json:"model"`
	Specifications  map[string]string `json:"specifications"`
	Snapshot        ItemSnapshotDTO   `json:"snapshot"`
	Fingerprint     string            `json:"fingerprint"`
	EngineeringKey  string            `json:"engineering_key"`
	Description     string            `json:"description"`
	SKU             string            `json:"sku"`
	Serial          int32             `json:"serial"`
	Status          string            `json:"status"`
	Active          bool              `json:"is_active"`
	Version         int32             `json:"version"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	CreatedBy       *string           `json:"created_by_id"`
	UpdatedBy       *string           `json:"updated_by_id"`
}

func ItemDTOFromDomain(v domain.Item) ItemDTO {
	return ItemDTO{
		ID:              v.ID,
		ConfigurationID: v.ConfigurationID,
		ManufacturerID:  v.ManufacturerID,
		Model:           v.Model,
		Specifications:  v.Specifications,
		Snapshot:        ItemSnapshotDTOFromDomain(v.Snapshot),
		Fingerprint:     v.Fingerprint,
		EngineeringKey:  v.EngineeringKey,
		Description:     v.Description,
		SKU:             v.SKU,
		Serial:          v.Serial,
		Status:          v.Status,
		Active:          v.Active,
		Version:         v.Version,
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
		CreatedBy:       v.CreatedBy,
		UpdatedBy:       v.UpdatedBy,
	}
}
func (v ItemDTO) Domain() domain.Item {
	return domain.Item{
		ID:              v.ID,
		ConfigurationID: v.ConfigurationID,
		ManufacturerID:  v.ManufacturerID,
		Model:           v.Model,
		Specifications:  v.Specifications,
		Snapshot:        v.Snapshot.Domain(),
		Fingerprint:     v.Fingerprint,
		EngineeringKey:  v.EngineeringKey,
		Description:     v.Description,
		SKU:             v.SKU,
		Serial:          v.Serial,
		Status:          v.Status,
		Active:          v.Active,
		Version:         v.Version,
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
		CreatedBy:       v.CreatedBy,
		UpdatedBy:       v.UpdatedBy,
	}
}

type ItemDefinitionDTO struct {
	Kind          string         `json:"kind"`
	CategoryCode  string         `json:"category_code"`
	FamilyCode    string         `json:"family_code"`
	UOM           string         `json:"uom"`
	Revision      int32          `json:"revision"`
	ModelRequired bool           `json:"model_required"`
	Fields        []ItemFieldDTO `json:"fields"`
}

func ItemDefinitionDTOFromDomain(v domain.ItemDefinition) ItemDefinitionDTO {
	return ItemDefinitionDTO{
		Kind:          v.Kind,
		CategoryCode:  v.CategoryCode,
		FamilyCode:    v.FamilyCode,
		UOM:           v.UOM,
		Revision:      v.Revision,
		ModelRequired: v.ModelRequired,
		Fields:        dtoSlice(v.Fields, ItemFieldDTOFromDomain),
	}
}
func (v ItemDefinitionDTO) Domain() domain.ItemDefinition {
	return domain.ItemDefinition{
		Kind:          v.Kind,
		CategoryCode:  v.CategoryCode,
		FamilyCode:    v.FamilyCode,
		UOM:           v.UOM,
		Revision:      v.Revision,
		ModelRequired: v.ModelRequired,
		Fields:        dtoSlice(v.Fields, ItemFieldDTO.Domain),
	}
}

type ItemFieldDTO struct {
	Code     string          `json:"code"`
	Label    string          `json:"label"`
	Required bool            `json:"required"`
	Options  []ItemOptionDTO `json:"options"`
}

func ItemFieldDTOFromDomain(v domain.ItemField) ItemFieldDTO {
	return ItemFieldDTO{
		Code:     v.Code,
		Label:    v.Label,
		Required: v.Required,
		Options:  dtoSlice(v.Options, ItemOptionDTOFromDomain),
	}
}
func (v ItemFieldDTO) Domain() domain.ItemField {
	return domain.ItemField{
		Code:     v.Code,
		Label:    v.Label,
		Required: v.Required,
		Options:  dtoSlice(v.Options, ItemOptionDTO.Domain),
	}
}

type ItemFrozenConfigurationDTO struct {
	ItemDefinitionDTO
	Name string `json:"name"`
	Code string `json:"code"`
}

func ItemFrozenConfigurationDTOFromDomain(v domain.ItemFrozenConfiguration) ItemFrozenConfigurationDTO {
	return ItemFrozenConfigurationDTO{
		ItemDefinitionDTO: ItemDefinitionDTOFromDomain(v.ItemDefinition),
		Name:              v.Name,
		Code:              v.Code,
	}
}
func (v ItemFrozenConfigurationDTO) Domain() domain.ItemFrozenConfiguration {
	return domain.ItemFrozenConfiguration{
		ItemDefinition: v.ItemDefinitionDTO.Domain(),
		Name:           v.Name,
		Code:           v.Code,
	}
}

type ItemMakeDTO struct {
	ID     string          `json:"id"`
	Code   string          `json:"code"`
	Name   string          `json:"name"`
	Active bool            `json:"is_active"`
	Data   ItemMakeDataDTO `json:"data"`
}

func ItemMakeDTOFromDomain(v domain.ItemMake) ItemMakeDTO {
	return ItemMakeDTO{
		ID:     v.ID,
		Code:   v.Code,
		Name:   v.Name,
		Active: v.Active,
		Data:   ItemMakeDataDTOFromDomain(v.Data),
	}
}
func (v ItemMakeDTO) Domain() domain.ItemMake {
	return domain.ItemMake{
		ID:     v.ID,
		Code:   v.Code,
		Name:   v.Name,
		Active: v.Active,
		Data:   v.Data.Domain(),
	}
}

type ItemMakeDataDTO struct {
	Eligible []string `json:"eligible"`
}

func ItemMakeDataDTOFromDomain(v domain.ItemMakeData) ItemMakeDataDTO {
	return ItemMakeDataDTO{
		Eligible: v.Eligible,
	}
}
func (v ItemMakeDataDTO) Domain() domain.ItemMakeData {
	return domain.ItemMakeData{
		Eligible: v.Eligible,
	}
}

type ItemOptionDTO struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

func ItemOptionDTOFromDomain(v domain.ItemOption) ItemOptionDTO {
	return ItemOptionDTO{
		Code:  v.Code,
		Label: v.Label,
	}
}
func (v ItemOptionDTO) Domain() domain.ItemOption {
	return domain.ItemOption{
		Code:  v.Code,
		Label: v.Label,
	}
}

type ItemSnapshotDTO struct {
	Configuration ItemFrozenConfigurationDTO `json:"configuration"`
	Manufacturer  ItemMakeDTO                `json:"manufacturer"`
	Prefix        string                     `json:"sku_prefix"`
}

func ItemSnapshotDTOFromDomain(v domain.ItemSnapshot) ItemSnapshotDTO {
	return ItemSnapshotDTO{
		Configuration: ItemFrozenConfigurationDTOFromDomain(v.Configuration),
		Manufacturer:  ItemMakeDTOFromDomain(v.Manufacturer),
		Prefix:        v.Prefix,
	}
}
func (v ItemSnapshotDTO) Domain() domain.ItemSnapshot {
	return domain.ItemSnapshot{
		Configuration: v.Configuration.Domain(),
		Manufacturer:  v.Manufacturer.Domain(),
		Prefix:        v.Prefix,
	}
}
