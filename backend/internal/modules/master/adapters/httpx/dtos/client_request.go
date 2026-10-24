package dtos

import (
	"encoding/json"
	"errors"
	"io"
	"regexp"

	"net/url"
	"strconv"
	"strings"

	"unicode/utf8"

	"github.com/ranakdinesh/epcERP/helpers"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func clientWebsite(s string) (string, bool) {
	s = masterDomain.ClientTrim(s)
	if s == "" {
		return s, true
	}
	if !clientScheme.MatchString(s) {
		s = "https://" + s
	}
	u, e := url.Parse(s)
	if e != nil || u.User != nil || !strings.Contains(u.Hostname(), ".") || masterDomain.ClientHasWhitespace(s) || (strings.ToLower(u.Scheme) != "http" && strings.ToLower(u.Scheme) != "https") {
		return s, false
	}
	if u.Port() != "" {
		p, e := strconv.Atoi(u.Port())
		if e != nil || p < 0 || p > 65535 {
			return s, false
		}
	}
	return s, true
}

func ParseClient(body io.Reader) (masterDomain.ClientInput, string, error) {
	var in masterDomain.ClientInput
	data, e := io.ReadAll(body)
	if e != nil {
		return in, "", e
	}
	m, e := StrictObject(data, "legal_name display_name parent_organization client_type industry client_type_id industry_id gstin pan cin website address_line_1 address_line_2 city state city_id state_id postal_code country contacts")
	if e != nil {
		return in, "", e
	}
	for _, f := range []struct {
		name     string
		dest     *string
		min, max int
		fallback string
	}{
		{"legal_name", &in.LegalName, 2, 255, ""},
		{"display_name", &in.DisplayName, 0, 255, ""},
		{"parent_organization", &in.ParentOrganization, 0, 255, ""},
		{"client_type", &in.ClientType, 0, 64, ""},
		{"industry", &in.Industry, 0, 128, ""},
		{"gstin", &in.Gstin, 1, 15, ""},
		{"pan", &in.Pan, 1, 10, ""},
		{"cin", &in.Cin, 0, 21, ""},
		{"website", &in.Website, 0, 255, ""},
		{"address_line_1", &in.AddressLine1, 1, 255, ""},
		{"address_line_2", &in.AddressLine2, 0, 255, ""},
		{"city", &in.City, 0, 128, ""},
		{"state", &in.State, 0, 128, ""},
		{"postal_code", &in.PostalCode, 6, 6, ""},
		{"country", &in.Country, 2, 128, "India"},
	} {
		raw, exists := m[f.name]
		v := f.fallback
		if exists {
			if f.name == "website" && string(raw) == "null" {
				v = ""
			} else if UnmarshalMasterJSON(raw, &v) != nil || string(raw) == "null" {
				return in, f.name, errors.New("string required")
			}
		} else if f.min > 0 && f.fallback == "" {
			return in, f.name, errors.New("required")
		}
		v = masterDomain.ClientTrim(v)
		if f.name == "website" {
			var valid bool
			v, valid = clientWebsite(v)
			if !valid {
				return in, f.name, errors.New("website")
			}
		}
		if n := utf8.RuneCountInString(v); n < f.min || n > f.max {
			return in, f.name, errors.New("length")
		}
		*f.dest = v
	}
	in.Gstin = strings.ToUpper(in.Gstin)
	in.Pan = strings.ToUpper(in.Pan)
	in.Cin = strings.ToUpper(in.Cin)
	if !masterDomain.ClientValidGST(in.Gstin) {
		return in, "gstin", errors.New("GSTIN")
	}
	if !masterDomain.ClientValidPAN(in.Pan) || in.Gstin[2:12] != in.Pan {
		return in, "pan", errors.New("PAN")
	}
	if in.Cin != "" && !clientCin.MatchString(in.Cin) {
		return in, "cin", errors.New("CIN")
	}
	if !clientPin.MatchString(in.PostalCode) {
		return in, "postal_code", errors.New("PIN")
	}
	for _, f := range []struct {
		name string
		dest *string
	}{{"client_type_id", &in.ClientTypeID}, {"industry_id", &in.IndustryID}, {"state_id", &in.StateID}, {"city_id", &in.CityID}} {
		var v string
		if UnmarshalMasterJSON(m[f.name], &v) != nil {
			return in, f.name, errors.New("UUID")
		}
		v, e = masterDomain.ClientCanonicalID(v)
		if e != nil {
			return in, f.name, e
		}
		*f.dest = v
	}
	var rows []json.RawMessage
	if UnmarshalMasterJSON(m["contacts"], &rows) != nil || string(m["contacts"]) == "null" || len(rows) > 20 {
		return in, "contacts", errors.New("contacts")
	}
	for _, raw := range rows {
		cm, e := StrictObject(raw, "id full_name designation department email phone is_primary contact_type")
		if e != nil {
			return in, "contacts", e
		}
		c := masterDomain.ClientContact{}
		for _, f := range []struct {
			name     string
			dest     *string
			min, max int
		}{{"full_name", &c.FullName, 2, 128}, {"contact_type", &c.Type, 0, 16}, {"designation", &c.Designation, 0, 128}, {"department", &c.Department, 0, 128}, {"email", &c.Email, 1, 255}, {"phone", &c.Phone, 1, 32}} {
			raw, exists := cm[f.name]
			v := ""
			if exists {
				if UnmarshalMasterJSON(raw, &v) != nil || string(raw) == "null" {
					return in, "contacts", errors.New("string")
				}
			}
			v = masterDomain.ClientTrim(v)
			n := utf8.RuneCountInString(v)
			if n < f.min || n > f.max {
				return in, "contacts", errors.New("length")
			}
			*f.dest = v
		}
		if !masterDomain.ClientValidContactType(c.Type) {
			return in, "contacts", errors.New("contact type")
		}
		if !clientEmail.MatchString(c.Email) || masterDomain.ClientHasWhitespace(c.Email) {
			return in, "contacts", errors.New("email")
		}
		c.Email = helpers.Lower(c.Email)
		phone := strings.NewReplacer(" ", "", "(", "", ")", "", "-", "").Replace(c.Phone)
		if !clientDomestic.MatchString(phone) && (!clientInternational.MatchString(phone) || strings.HasPrefix(phone, "+91")) {
			return in, "contacts", errors.New("phone")
		}
		if raw, ok := cm["id"]; ok && string(raw) != "null" {
			var v string
			if UnmarshalMasterJSON(raw, &v) != nil {
				return in, "contacts", errors.New("UUID")
			}
			c.ID, e = masterDomain.ClientCanonicalID(v)
			if e != nil {
				return in, "contacts", e
			}
		}
		if raw, ok := cm["is_primary"]; ok {
			var valid bool
			c.Primary, valid = JsonBoolean(raw)
			if !valid {
				return in, "contacts", errors.New("bool")
			}
		}
		in.Contacts = append(in.Contacts, c)
	}
	return in, "", nil
}

var clientEmail = regexp.MustCompile(`^[^\s@]+@[^\s@.]+(?:\.[^\s@.]+)+$`)
var clientDomestic = regexp.MustCompile(`^(?:\+91|0)?[1-9][0-9]{9}$`)
var clientInternational = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
var clientCin = regexp.MustCompile(`^[A-Z0-9]{21}$`)
var clientPin = regexp.MustCompile(`^[1-9][0-9]{5}$`)
var clientScheme = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*://`)
