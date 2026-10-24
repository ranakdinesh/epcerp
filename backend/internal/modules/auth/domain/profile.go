package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

var ErrProfileInvalid = &Error{Kind: Invalid, Detail: "invalid profile or photo"}

const ProfileMaximumPhotoBytes = 512 * 1024

type Profile struct {
	UserID, DisplayName       string
	Contact                   *PersonalContact
	Photo                     []byte // Legacy database photo; new uploads use encrypted object storage.
	PhotoObjectKey, PhotoHash string
	Version                   int64
	CreatedAt, UpdatedAt      time.Time
}
type ProfileChange struct {
	ID, UserID, Action string
	Version            int64
	At                 time.Time
	Before, After      *PersonalContact
	Reason             string
}

func ProfileName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > 100 || strings.ContainsAny(s, "\r\n\x00") {
		return "", ErrProfileInvalid
	}
	return s, nil
}

// PersonalContact is exposed only to the signed-in account holder.
type PersonalContact struct{ FirstName, LastName, Mobile string }
type PersonalDetails struct {
	Name              string
	Contact, Expected PersonalContact
	Version           int64
}

var ErrProfileConflict = &Error{Kind: Conflict, Detail: "Your profile changed. Close and reopen it before saving."}

func (p PersonalDetails) Validate() error {
	if !ValidEmployeeContact(p.Contact.FirstName, p.Contact.LastName, p.Contact.Mobile) || p.Version < 0 {
		return &Error{Kind: Invalid, Detail: "Provide first name, last name, a valid mobile number."}
	}
	_, err := ProfileName(p.Name)
	return err
}
