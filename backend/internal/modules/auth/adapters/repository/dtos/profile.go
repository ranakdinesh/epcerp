package dtos

import (
	"encoding/json"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/adapters/repository/sqlc"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

func Profile(v sqlc.AuthUserProfile) domain.Profile {
	return domain.Profile{UserID: v.UserID.String(), DisplayName: v.DisplayName, Photo: v.Photo, PhotoObjectKey: v.PhotoObjectKey.String, PhotoHash: v.PhotoSha256.String, Version: v.Version, CreatedAt: v.CreatedAt.Time, UpdatedAt: v.UpdatedAt.Time}
}

func ProfileSave(id string, name *string, key, hash string, replace bool) sqlc.SaveProfileParams {
	params := sqlc.SaveProfileParams{UserID: UUID(id), DisplayName: Nullable(name), ReplacePhoto: replace}
	if key != "" {
		params.PhotoObjectKey = Nullable(&key)
		params.PhotoSha256 = Nullable(&hash)
	}
	return params
}

// ContactSnapshot is the stable, private persistence representation of a contact.
func ContactSnapshot(c *domain.PersonalContact) ([]byte, error) {
	return json.Marshal(struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Mobile    string `json:"mobile"`
	}{c.FirstName, c.LastName, c.Mobile})
}
