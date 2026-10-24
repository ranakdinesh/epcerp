package dtos

import (
	"encoding/json"
	"io"

	"github.com/ranakdinesh/epcERP/helpers"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type ProfileResponse struct {
	ID      string           `json:"user_id"`
	Name    string           `json:"display_name"`
	Photo   bool             `json:"has_photo"`
	Version int64            `json:"version"`
	Contact *PersonalContact `json:"contact,omitempty"`
}

func FromProfile(p domain.Profile) ProfileResponse {
	var contact *PersonalContact
	if p.Contact != nil {
		contact = &PersonalContact{p.Contact.FirstName, p.Contact.LastName, p.Contact.Mobile}
	}
	return ProfileResponse{p.UserID, p.DisplayName, len(p.Photo) > 0 || p.PhotoObjectKey != "", p.Version, contact}
}

type ProfileUpdateRequest struct {
	Name     *string          `json:"display_name"`
	Contact  *PersonalContact `json:"contact,omitempty"`
	Expected *PersonalContact `json:"expected_contact,omitempty"`
	Version  *int64           `json:"version,omitempty"`
}

func ParseProfileUpdate(r io.Reader) (ProfileUpdateRequest, error) {
	var p ProfileUpdateRequest
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil || p.Name == nil || d.Decode(&struct{}{}) != io.EOF {
		return p, domain.ErrProfileInvalid
	}
	if p.Contact != nil && (p.Expected == nil || p.Version == nil) {
		return p, domain.ErrProfileInvalid
	}
	if p.Contact == nil && (p.Expected != nil || p.Version != nil) {
		return p, domain.ErrProfileInvalid
	}
	return p, nil
}
func ProfileID(v string) (string, error) { return helpers.CanonicalID(v) }

type PersonalContact struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Mobile    string `json:"mobile"`
}

func (p ProfileUpdateRequest) Details() domain.PersonalDetails {
	return domain.PersonalDetails{Name: *p.Name, Contact: domain.PersonalContact{FirstName: p.Contact.FirstName, LastName: p.Contact.LastName, Mobile: p.Contact.Mobile}, Expected: domain.PersonalContact{FirstName: p.Expected.FirstName, LastName: p.Expected.LastName, Mobile: p.Expected.Mobile}, Version: *p.Version}
}
