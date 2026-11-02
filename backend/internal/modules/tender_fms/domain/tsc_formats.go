package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const DefaultTSCFormatID = "00000000-0000-4000-8000-000000000097"

type TSCFormat struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Purpose       string        `json:"purpose"`
	CreatedAt     time.Time     `json:"created_at"`
	Description   string        `json:"description"`
	Active        bool          `json:"is_active"`
	Version       int64         `json:"version"`
	QuestionCount int64         `json:"question_count"`
	Questions     []TSCQuestion `json:"questions,omitempty"`
	UpdatedAt     time.Time     `json:"updated_at"`
}
type TSCFormatInput struct {
	Name        string        `json:"name"`
	Purpose     string        `json:"purpose"`
	Description string        `json:"description"`
	Active      bool          `json:"is_active"`
	Version     int64         `json:"expected_version"`
	Questions   []TSCQuestion `json:"questions"`
}
type TSCFormatSelection struct {
	ID      string `json:"format_id"`
	Version int64  `json:"format_version"`
}

func TSCQuestionKey(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, text)
}

var tscQuestionID = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,80}$`)

func ValidateTSCFormat(in *TSCFormatInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Purpose = strings.TrimSpace(in.Purpose)
	if in.Purpose == "" {
		in.Purpose = "Generic"
	}
	if utf8.RuneCountInString(in.Purpose) > 80 {
		return Invalid("Purpose must be at most 80 characters")
	}
	if n := utf8.RuneCountInString(in.Name); n < 3 || n > 120 || TSCQuestionKey(in.Name) == "" {
		return Invalid("Give this TSC format a name of 3–120 characters")
	}
	if utf8.RuneCountInString(in.Description) > 1000 {
		return Invalid("Format description must be at most 1000 characters")
	}
	if len(in.Questions) < 1 || len(in.Questions) > 200 {
		return Invalid("A TSC format must contain 1–200 questions")
	}
	seen, ids := map[string]int{}, map[string]bool{}
	for i := range in.Questions {
		q := &in.Questions[i]
		q.Text = strings.TrimSpace(q.Text)
		q.Section = strings.TrimSpace(q.Section)
		q.Number = strings.TrimSpace(q.Number)
		if n := utf8.RuneCountInString(q.Text); n < 3 || n > 2000 || TSCQuestionKey(q.Text) == "" {
			return Invalid(fmt.Sprintf("Question %d must contain 3–2000 characters", i+1))
		}
		if n := utf8.RuneCountInString(q.Section); n < 1 || n > 80 {
			return Invalid(fmt.Sprintf("Give question %d a section of 1–80 characters", i+1))
		}
		if utf8.RuneCountInString(q.Number) > 30 {
			return Invalid("Question numbers must be at most 30 characters")
		}
		key := TSCQuestionKey(q.Text)
		if previous, ok := seen[key]; ok {
			return Invalid(fmt.Sprintf("Question %d duplicates question %d. Each question must be unique within this TSC format", i+1, previous))
		}
		seen[key] = i + 1
		if !tscQuestionID.MatchString(q.ID) || ids[q.ID] {
			return Invalid("Question identities must be valid and unique")
		}
		ids[q.ID] = true
	}
	return nil
}
