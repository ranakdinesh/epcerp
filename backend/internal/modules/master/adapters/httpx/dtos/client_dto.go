package dtos

import (
	"time"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ClientDTO struct {
	ID                 string             `json:"id"`
	Code               string             `json:"client_code"`
	LegalName          string             `json:"legal_name"`
	DisplayName        string             `json:"display_name"`
	ParentOrganization string             `json:"parent_organization"`
	ClientType         string             `json:"client_type"`
	Industry           string             `json:"industry"`
	Gstin              string             `json:"gstin"`
	Pan                string             `json:"pan"`
	Cin                string             `json:"cin"`
	Website            string             `json:"website"`
	AddressLine1       string             `json:"address_line_1"`
	AddressLine2       string             `json:"address_line_2"`
	City               string             `json:"city"`
	State              string             `json:"state"`
	PostalCode         string             `json:"postal_code"`
	Country            string             `json:"country"`
	Active             bool               `json:"is_active"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	Contacts           []ClientContactDTO `json:"contacts"`
}

func ClientDTOFromDomain(v masterDomain.Client) ClientDTO {
	return ClientDTO{
		ID:                 v.ID,
		Code:               v.Code,
		LegalName:          v.LegalName,
		DisplayName:        v.DisplayName,
		ParentOrganization: v.ParentOrganization,
		ClientType:         v.ClientType,
		Industry:           v.Industry,
		Gstin:              v.Gstin,
		Pan:                v.Pan,
		Cin:                v.Cin,
		Website:            v.Website,
		AddressLine1:       v.AddressLine1,
		AddressLine2:       v.AddressLine2,
		City:               v.City,
		State:              v.State,
		PostalCode:         v.PostalCode,
		Country:            v.Country,
		Active:             v.Active,
		CreatedAt:          v.CreatedAt,
		UpdatedAt:          v.UpdatedAt,
		Contacts:           dtoSlice(v.Contacts, ClientContactDTOFromDomain),
	}
}
func (v ClientDTO) Domain() masterDomain.Client {
	return masterDomain.Client{
		ID:                 v.ID,
		Code:               v.Code,
		LegalName:          v.LegalName,
		DisplayName:        v.DisplayName,
		ParentOrganization: v.ParentOrganization,
		ClientType:         v.ClientType,
		Industry:           v.Industry,
		Gstin:              v.Gstin,
		Pan:                v.Pan,
		Cin:                v.Cin,
		Website:            v.Website,
		AddressLine1:       v.AddressLine1,
		AddressLine2:       v.AddressLine2,
		City:               v.City,
		State:              v.State,
		PostalCode:         v.PostalCode,
		Country:            v.Country,
		Active:             v.Active,
		CreatedAt:          v.CreatedAt,
		UpdatedAt:          v.UpdatedAt,
		Contacts:           dtoSlice(v.Contacts, ClientContactDTO.Domain),
	}
}

type ClientContactDTO struct {
	Type        string `json:"contact_type"`
	ID          string `json:"id"`
	FullName    string `json:"full_name"`
	Designation string `json:"designation"`
	Department  string `json:"department"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Primary     bool   `json:"is_primary"`
	Active      bool   `json:"is_active"`
}

func ClientContactDTOFromDomain(v masterDomain.ClientContact) ClientContactDTO {
	return ClientContactDTO{
		Type:        v.Type,
		ID:          v.ID,
		FullName:    v.FullName,
		Designation: v.Designation,
		Department:  v.Department,
		Email:       v.Email,
		Phone:       v.Phone,
		Primary:     v.Primary,
		Active:      v.Active,
	}
}
func (v ClientContactDTO) Domain() masterDomain.ClientContact {
	return masterDomain.ClientContact{
		Type:        v.Type,
		ID:          v.ID,
		FullName:    v.FullName,
		Designation: v.Designation,
		Department:  v.Department,
		Email:       v.Email,
		Phone:       v.Phone,
		Primary:     v.Primary,
		Active:      v.Active,
	}
}

type ClientPageDTO struct {
	Items      []ClientDTO `json:"items"`
	Page       int32       `json:"page"`
	Size       int32       `json:"page_size"`
	Total      int64       `json:"total"`
	TotalPages int64       `json:"total_pages"`
	All        int64       `json:"all_count"`
	Active     int64       `json:"active_count"`
	Inactive   int64       `json:"inactive_count"`
}

func ClientPageDTOFromDomain(v masterDomain.ClientPage) ClientPageDTO {
	return ClientPageDTO{
		Items:      dtoSlice(v.Items, ClientDTOFromDomain),
		Page:       v.Page,
		Size:       v.Size,
		Total:      v.Total,
		TotalPages: v.TotalPages,
		All:        v.All,
		Active:     v.Active,
		Inactive:   v.Inactive,
	}
}
func (v ClientPageDTO) Domain() masterDomain.ClientPage {
	return masterDomain.ClientPage{
		Items:      dtoSlice(v.Items, ClientDTO.Domain),
		Page:       v.Page,
		Size:       v.Size,
		Total:      v.Total,
		TotalPages: v.TotalPages,
		All:        v.All,
		Active:     v.Active,
		Inactive:   v.Inactive,
	}
}

type ClientClientResponse struct {
	ClientDTO
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
	MissingContactTypes []string `json:"missing_contact_types"`
}

func ClientTimestamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func ClientResponse(c masterDomain.Client) ClientClientResponse {
	return ClientClientResponse{ClientDTO: ClientDTOFromDomain(c), CreatedAt: ClientTimestamp(c.CreatedAt), UpdatedAt: ClientTimestamp(c.UpdatedAt), MissingContactTypes: masterDomain.ClientMissingContactTypes(c.Contacts)}
}

type ClientPageResponse struct {
	ClientPageDTO
	Items []ClientClientResponse `json:"items"`
}

func ClientPage(value masterDomain.ClientPage) ClientPageResponse {
	items := make([]ClientClientResponse, 0, len(value.Items))
	for _, c := range value.Items {
		items = append(items, ClientResponse(c))
	}
	return ClientPageResponse{ClientPageDTO: ClientPageDTOFromDomain(value), Items: items}
}
