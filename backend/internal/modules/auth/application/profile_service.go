package application

import (
	"context"

	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/ports"
)

type ProfileService struct {
	unit     ports.UnitOfWork[ports.ProfileRepository]
	identity ports.ProfileIdentity
	images   ports.ImageProcessor
	values   ports.ProfileValues
	storage  ports.ImageStorage
}

func NewProfile(u ports.UnitOfWork[ports.ProfileRepository], i ports.ProfileIdentity, im ports.ImageProcessor, v ports.ProfileValues, storage ports.ImageStorage) *ProfileService {
	return &ProfileService{u, i, im, v, storage}
}
func (s *ProfileService) principal(ctx context.Context, token, permission string) (sharedIdentity.Principal, error) {
	p, e := s.identity.Resolve(ctx, token)
	if e != nil {
		return p, e
	}
	if !p.Active || p.ID == "" {
		return p, domain.ErrUnauthorized
	}
	if !p.Allows(permission) {
		return p, domain.ErrForbidden
	}
	return p, nil
}
func (s *ProfileService) Get(ctx context.Context, token, target string) (domain.Profile, error) {
	p, e := s.principal(ctx, token, "account.profile.read")
	if e != nil {
		return domain.Profile{}, e
	}
	if target == "" {
		target = p.ID
	}
	var result domain.Profile
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.ProfileRepository) error {
		user, e := r.User(ctx, target, false)
		if e != nil {
			return e
		}
		if user == nil || !user.IsActive {
			return domain.ErrUserMissing
		}
		v, e := r.Get(ctx, target)
		if e != nil {
			return e
		}
		if v == nil {
			result = domain.Profile{UserID: target, DisplayName: user.Username}
		} else {
			result = *v
		}
		if target == p.ID {
			result.Contact = &domain.PersonalContact{FirstName: user.FirstName, LastName: user.LastName, Mobile: user.Mobile}
		}
		if result.DisplayName == "" {
			result.DisplayName = user.Username
		}
		return nil
	})
	return result, e
}
func (s *ProfileService) Update(ctx context.Context, token string, name *string, photo []byte, replace bool) (domain.Profile, error) {
	p, e := s.principal(ctx, token, "account.profile.write")
	if e != nil {
		return domain.Profile{}, e
	}
	if name != nil {
		v, e := domain.ProfileName(*name)
		if e != nil {
			return domain.Profile{}, e
		}
		name = &v
	}
	if replace && len(photo) > 0 {
		photo, e = s.images.Canonical(photo)
		if e != nil {
			return domain.Profile{}, domain.ErrProfileInvalid
		}
	}
	var result domain.Profile
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.ProfileRepository) error {
		user, e := r.User(ctx, p.ID, true)
		if e != nil {
			return e
		}
		if user == nil || !user.IsActive {
			return domain.ErrUnauthorized
		}
		key, hash := "", ""
		if replace && len(photo) > 0 {
			key = "images/auth/profiles/" + p.ID + "/" + s.values.ID() + ".enc"
			hash, e = s.storage.Store(ctx, key, photo)
			if e != nil {
				return e
			}
		}
		// Objects are immutable. Never delete on a database commit error: its result
		// can be ambiguous, and the uploaded object may already be referenced.
		result, e = r.Save(ctx, p.ID, name, key, hash, replace)
		if e != nil {
			return e
		}
		action := "details_updated"
		if replace {
			action = "photo_updated"
			if len(photo) == 0 {
				action = "photo_removed"
			}
		}
		return r.Record(ctx, domain.ProfileChange{ID: s.values.ID(), UserID: p.ID, Action: action, Version: result.Version, At: result.UpdatedAt})
	})
	return result, e
}

func (s *ProfileService) Image(ctx context.Context, token, target string) ([]byte, error) {
	p, err := s.Get(ctx, token, target)
	if err != nil {
		return nil, err
	}
	if p.PhotoObjectKey != "" {
		return s.storage.Retrieve(ctx, p.PhotoObjectKey, p.PhotoHash)
	}
	return p.Photo, nil
}

// UpdateDetails atomically updates the canonical employee contact and presentation.
// Expected contact values also detect an administrator's intervening correction.
func (s *ProfileService) UpdateDetails(ctx context.Context, token string, in domain.PersonalDetails) (domain.Profile, error) {
	p, e := s.principal(ctx, token, "account.profile.write")
	if e != nil {
		return domain.Profile{}, e
	}
	if e = in.Validate(); e != nil {
		return domain.Profile{}, e
	}
	name, e := domain.ProfileName(in.Name)
	if e != nil {
		return domain.Profile{}, e
	}
	var result domain.Profile
	e = s.unit.Within(ctx, func(ctx context.Context, r ports.ProfileRepository) error {
		if e := r.Lock(ctx, "auth.administration"); e != nil {
			return e
		}
		user, e := r.User(ctx, p.ID, true)
		if e != nil {
			return e
		}
		if user == nil || !user.IsActive {
			return domain.ErrUnauthorized
		}
		before := domain.PersonalContact{FirstName: user.FirstName, LastName: user.LastName, Mobile: user.Mobile}
		current, e := r.Get(ctx, p.ID)
		if e != nil {
			return e
		}
		version := int64(0)
		if current != nil {
			version = current.Version
		}
		if before != in.Expected || version != in.Version {
			return domain.ErrProfileConflict
		}
		eventID := s.values.ID()
		const summary = "Account holder updated personal profile."
		if e = r.SaveContact(ctx, p.ID, in.Contact, eventID, summary); e != nil {
			return e
		}
		result, e = r.Save(ctx, p.ID, &name, "", "", false)
		if e != nil {
			return e
		}
		result.Contact = &in.Contact
		return r.Record(ctx, domain.ProfileChange{ID: eventID, UserID: p.ID, Action: "personal_details_updated", Version: result.Version, At: result.UpdatedAt, Before: &before, After: &in.Contact, Reason: summary})
	})
	return result, e
}
