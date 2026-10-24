package domain

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type DocumentDraft struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	OwnerID   string       `json:"-"`
	Version   int64        `json:"version"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	Batches   []DraftBatch `json:"batches"`
}
type DraftDisposition struct {
	Path  string `json:"path"`
	State string `json:"state"`
	Code  string `json:"code"`
}
type DraftBatch struct {
	Manifest  []DraftDisposition `json:"manifest"`
	ID        string             `json:"id"`
	Revision  string             `json:"revision"`
	State     string             `json:"state"`
	Error     string             `json:"error"`
	Version   int64              `json:"version"`
	Attempts  int32              `json:"attempts"`
	UpdatedAt time.Time          `json:"updated_at"`
	Files     []DraftFile        `json:"files"`
	Sources   []DraftSource      `json:"sources"`
}
type DraftFile struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	State   string `json:"state"`
	Error   string `json:"error"`
	SHA256  string `json:"sha256"`
	Version int64  `json:"version"`
}
type DraftSource struct {
	ID             string    `json:"id"`
	MemberID       string    `json:"member_id"`
	Path           string    `json:"path"`
	Filename       string    `json:"filename"`
	SHA256         string    `json:"sha256"`
	Status         string    `json:"status"`
	Error          string    `json:"error"`
	Version        int32     `json:"version"`
	Attempts       int32     `json:"attempts"`
	Segments       int32     `json:"segments"`
	ExpectedPages  int32     `json:"expected_pages"`
	ExpectedSheets int32     `json:"expected_sheets"`
	Warnings       []string  `json:"warnings"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func DraftName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) < 2 || utf8.RuneCountInString(s) > 255 {
		return s, Invalid("Tender name must contain 2 to 255 characters")
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return s, Invalid("Tender name cannot contain control characters")
		}
	}
	return s, nil
}

type DraftQuery struct {
	Search              string
	SortName, Ascending bool
	Limit, Offset       int32
}
