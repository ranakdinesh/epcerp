package dtos

import (
	"time"

	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type VendorDTO struct {
	ID        string         `json:"id"`
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Data      map[string]any `json:"data"`
	Active    bool           `json:"is_active"`
	Version   int32          `json:"version"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	CreatedBy *string        `json:"created_by_id"`
	UpdatedBy *string        `json:"updated_by_id"`
	Imported  bool           `json:"is_imported"`
}

func VendorDTOFromDomain(v domain.Vendor) VendorDTO {
	return VendorDTO{
		ID:        v.ID,
		Code:      v.Code,
		Name:      v.Name,
		Data:      v.Data,
		Active:    v.Active,
		Version:   v.Version,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
		CreatedBy: v.CreatedBy,
		UpdatedBy: v.UpdatedBy,
		Imported:  v.Imported,
	}
}
func (v VendorDTO) Domain() domain.Vendor {
	return domain.Vendor{
		ID:        v.ID,
		Code:      v.Code,
		Name:      v.Name,
		Data:      v.Data,
		Active:    v.Active,
		Version:   v.Version,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
		CreatedBy: v.CreatedBy,
		UpdatedBy: v.UpdatedBy,
		Imported:  v.Imported,
	}
}

type VendorCreateDTO struct {
	Code   string         `json:"code"`
	Name   string         `json:"name"`
	Data   map[string]any `json:"data"`
	Reason string         `json:"reason"`
}

func VendorCreateDTOFromDomain(v domain.VendorCreate) VendorCreateDTO {
	return VendorCreateDTO{
		Code:   v.Code,
		Name:   v.Name,
		Data:   v.Data,
		Reason: v.Reason,
	}
}
func (v VendorCreateDTO) Domain() domain.VendorCreate {
	return domain.VendorCreate{
		Code:   v.Code,
		Name:   v.Name,
		Data:   v.Data,
		Reason: v.Reason,
	}
}

type VendorInputDTO struct {
	Source VendorSourceDTO `json:"source"`
	Code   string          `json:"code"`
	Name   string          `json:"name"`
	Data   map[string]any  `json:"data"`
	Active *bool           `json:"is_active"`
}

func VendorInputDTOFromDomain(v domain.VendorInput) VendorInputDTO {
	return VendorInputDTO{
		Source: VendorSourceDTOFromDomain(v.Source),
		Code:   v.Code,
		Name:   v.Name,
		Data:   v.Data,
		Active: v.Active,
	}
}
func (v VendorInputDTO) Domain() domain.VendorInput {
	return domain.VendorInput{
		Source: v.Source.Domain(),
		Code:   v.Code,
		Name:   v.Name,
		Data:   v.Data,
		Active: v.Active,
	}
}

type VendorSourceDTO struct {
	ID        string    `json:"id"`
	Version   int32     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func VendorSourceDTOFromDomain(v domain.VendorSource) VendorSourceDTO {
	return VendorSourceDTO{
		ID:        v.ID,
		Version:   v.Version,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}
func (v VendorSourceDTO) Domain() domain.VendorSource {
	return domain.VendorSource{
		ID:        v.ID,
		Version:   v.Version,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

type VendorUpdateDTO struct {
	Name    string         `json:"name"`
	Data    map[string]any `json:"data"`
	Version int32          `json:"version"`
	Active  *bool          `json:"is_active"`
	Reason  string         `json:"reason"`
}

func VendorUpdateDTOFromDomain(v domain.VendorUpdate) VendorUpdateDTO {
	return VendorUpdateDTO{
		Name:    v.Name,
		Data:    v.Data,
		Version: v.Version,
		Active:  v.Active,
		Reason:  v.Reason,
	}
}
func (v VendorUpdateDTO) Domain() domain.VendorUpdate {
	return domain.VendorUpdate{
		Name:    v.Name,
		Data:    v.Data,
		Version: v.Version,
		Active:  v.Active,
		Reason:  v.Reason,
	}
}
