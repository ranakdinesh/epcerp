package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var Unauthorized = errors.New("Authentication required")
var Forbidden = errors.New("Calendar access denied")
var Invalid = errors.New("Invalid holiday name, date, reason or current version")
var Conflict = errors.New("Holiday changed or date already exists; refresh before saving")

type Holiday struct {
	ID, Name, Date, Actor string
	Active                bool
	Version               int32
}
type Change struct {
	ID, Name, Date, Reason string
	Active                 bool
	Version                int32
}
type Count struct{ CalendarDays, WeekendDays, HolidayDays, WorkingDays int }

func ValidDate(s string) (time.Time, error) {
	v, e := time.Parse("2006-01-02", s)
	if e != nil || v.Year() < 1 || v.Year() > 9999 {
		return v, Invalid
	}
	return v, nil
}
func (c Change) Validate() (Change, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.Reason = strings.TrimSpace(c.Reason)
	if utf8.RuneCountInString(c.Name) < 2 || utf8.RuneCountInString(c.Name) > 128 || c.Reason == "" || utf8.RuneCountInString(c.Reason) > 2000 || c.Version < 0 || (c.ID == "" && c.Version != 0) || (c.ID != "" && c.Version < 1) {
		return c, Invalid
	}
	if _, e := ValidDate(c.Date); e != nil {
		return c, e
	}
	return c, nil
}
func Working(start, end string, holidays map[string]bool) (Count, error) {
	s, e := ValidDate(start)
	if e != nil {
		return Count{}, e
	}
	f, e := ValidDate(end)
	if e != nil {
		return Count{}, e
	}
	if f.Before(s) {
		return Count{}, nil
	}
	if f.Sub(s) > 36500*24*time.Hour {
		return Count{}, Invalid
	}
	r := Count{}
	for d := s; !d.After(f); d = d.AddDate(0, 0, 1) {
		r.CalendarDays++
		n := (d.Day()-1)/7 + 1
		if d.Weekday() == time.Sunday || (d.Weekday() == time.Saturday && (n == 2 || n == 4)) {
			r.WeekendDays++
		} else if holidays[d.Format("2006-01-02")] {
			r.HolidayDays++
		} else {
			r.WorkingDays++
		}
	}
	return r, nil
}
