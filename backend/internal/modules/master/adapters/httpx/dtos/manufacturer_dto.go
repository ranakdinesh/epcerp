package dtos

import (
	"time"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ManufacturerChoiceDTO struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	ParentName string `json:"parent_name"`
	ParentCode string `json:"parent_code"`
	Active     bool   `json:"is_active"`
}

func ManufacturerChoiceDTOFromDomain(v masterDomain.ManufacturerChoice) ManufacturerChoiceDTO {
	return ManufacturerChoiceDTO{
		Code:       v.Code,
		Name:       v.Name,
		Kind:       v.Kind,
		ParentName: v.ParentName,
		ParentCode: v.ParentCode,
		Active:     v.Active,
	}
}
func (v ManufacturerChoiceDTO) Domain() masterDomain.ManufacturerChoice {
	return masterDomain.ManufacturerChoice{
		Code:       v.Code,
		Name:       v.Name,
		Kind:       v.Kind,
		ParentName: v.ParentName,
		ParentCode: v.ParentCode,
		Active:     v.Active,
	}
}

type ManufacturerDataDTO struct {
	Eligible []string `json:"eligible"`
	Notes    string   `json:"notes"`
}

func ManufacturerDataDTOFromDomain(v masterDomain.ManufacturerData) ManufacturerDataDTO {
	return ManufacturerDataDTO{
		Eligible: v.Eligible,
		Notes:    v.Notes,
	}
}
func (v ManufacturerDataDTO) Domain() masterDomain.ManufacturerData {
	return masterDomain.ManufacturerData{
		Eligible: v.Eligible,
		Notes:    v.Notes,
	}
}

type ManufacturerLegacyDTO struct {
	Source ManufacturerLegacySourceDTO `json:"source"`
	Code   string                      `json:"code"`
	Name   string                      `json:"name"`
	Data   ManufacturerDataDTO         `json:"data"`
	Active *bool                       `json:"is_active"`
}

func ManufacturerLegacyDTOFromDomain(v masterDomain.ManufacturerLegacy) ManufacturerLegacyDTO {
	return ManufacturerLegacyDTO{
		Source: ManufacturerLegacySourceDTOFromDomain(v.Source),
		Code:   v.Code,
		Name:   v.Name,
		Data:   ManufacturerDataDTOFromDomain(v.Data),
		Active: v.Active,
	}
}
func (v ManufacturerLegacyDTO) Domain() masterDomain.ManufacturerLegacy {
	return masterDomain.ManufacturerLegacy{
		Source: v.Source.Domain(),
		Code:   v.Code,
		Name:   v.Name,
		Data:   v.Data.Domain(),
		Active: v.Active,
	}
}

type ManufacturerLegacySourceDTO struct {
	CreatedBy *string   `json:"created_by_id"`
	UpdatedBy *string   `json:"updated_by_id"`
	ID        string    `json:"id"`
	Version   int32     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ManufacturerLegacySourceDTOFromDomain(v masterDomain.ManufacturerLegacySource) ManufacturerLegacySourceDTO {
	return ManufacturerLegacySourceDTO{
		CreatedBy: v.CreatedBy,
		UpdatedBy: v.UpdatedBy,
		ID:        v.ID,
		Version:   v.Version,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}
func (v ManufacturerLegacySourceDTO) Domain() masterDomain.ManufacturerLegacySource {
	return masterDomain.ManufacturerLegacySource{
		CreatedBy: v.CreatedBy,
		UpdatedBy: v.UpdatedBy,
		ID:        v.ID,
		Version:   v.Version,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

type ManufacturerSelectionDTO struct {
	Rows  []ManufacturerChoiceDTO `json:"rows"`
	Total int64                   `json:"total"`
}

func ManufacturerSelectionDTOFromDomain(v masterDomain.ManufacturerSelection) ManufacturerSelectionDTO {
	return ManufacturerSelectionDTO{
		Rows:  dtoSlice(v.Rows, ManufacturerChoiceDTOFromDomain),
		Total: v.Total,
	}
}
func (v ManufacturerSelectionDTO) Domain() masterDomain.ManufacturerSelection {
	return masterDomain.ManufacturerSelection{
		Rows:  dtoSlice(v.Rows, ManufacturerChoiceDTO.Domain),
		Total: v.Total,
	}
}
