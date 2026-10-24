package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/ranakdinesh/epcERP/helpers"
)

var ClientUnauthorized = &Error{Kind: Unauthenticated, Detail: "Invalid or expired credentials"}
var ClientForbidden = &Error{Kind: AccessDenied, Detail: "Insufficient permission"}
var ClientMissing = &Error{Kind: NotFound, Detail: "Client not found"}
var ClientDuplicate = &Error{Kind: Conflict, Detail: "A client with this legal name already exists"}

func ClientInvalid(s string) error { return &Error{Kind: InvalidInput, Detail: s} }

type ClientContact struct {
	Type        string
	ID          string
	FullName    string
	Designation string
	Department  string
	Email       string
	Phone       string
	Primary     bool
	Active      bool
}
type Client struct {
	ID                 string
	Code               string
	LegalName          string
	DisplayName        string
	ParentOrganization string
	ClientType         string
	Industry           string
	Gstin              string
	Pan                string
	Cin                string
	Website            string
	AddressLine1       string
	AddressLine2       string
	City               string
	State              string
	PostalCode         string
	Country            string

	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
	Contacts  []ClientContact
	// Retain provenance and actor data internally without expanding the public response.
	NormalizedName       string
	CreatedBy, UpdatedBy *string
}
type ClientInput struct {
	LegalName          string
	DisplayName        string
	ParentOrganization string
	ClientType         string
	Industry           string
	Gstin              string
	Pan                string
	Cin                string
	Website            string
	AddressLine1       string
	AddressLine2       string
	City               string
	State              string
	PostalCode         string
	Country            string

	ClientTypeID, IndustryID, CityID, StateID string
	Contacts                                  []ClientContact
}
type ClientReference struct {
	ID, Name string
	Active   bool
	StateID  string
}
type ClientFilter struct {
	Page, Size               int32
	Search, ClientType, Sort string
	Active                   *bool
	Descending               bool
}
type ClientPage struct {
	Items      []Client
	Page       int32
	Size       int32
	Total      int64
	TotalPages int64
	All        int64
	Active     int64
	Inactive   int64
}

func ClientNormalize(s string) string {
	return helpers.Fold(strings.Join(strings.FieldsFunc(s, clientWhitespace), " "))
}
func ClientPrepare(in ClientInput, c *Client) error {
	if len(in.Contacts) > 20 {
		return ClientInvalid("At most 20 contacts are permitted")
	}
	primary := 0
	for _, v := range in.Contacts {
		if !ClientValidContactType(v.Type) {
			return ClientInvalid("Invalid contact type")
		}
		if v.Primary {
			primary++
		}
	}
	if primary > 1 {
		return ClientInvalid("Only one primary contact can be assigned to a client")
	}
	c.LegalName = ClientTrim(in.LegalName)
	c.DisplayName = ClientTrim(in.DisplayName)
	c.ParentOrganization = ClientTrim(in.ParentOrganization)
	c.ClientType = ClientTrim(in.ClientType)
	c.Industry = ClientTrim(in.Industry)
	c.Gstin = ClientTrim(in.Gstin)
	c.Pan = ClientTrim(in.Pan)
	c.Cin = ClientTrim(in.Cin)
	c.Website = ClientTrim(in.Website)
	c.AddressLine1 = ClientTrim(in.AddressLine1)
	c.AddressLine2 = ClientTrim(in.AddressLine2)
	c.City = ClientTrim(in.City)
	c.State = ClientTrim(in.State)
	c.PostalCode = ClientTrim(in.PostalCode)
	c.Country = ClientTrim(in.Country)

	c.NormalizedName = ClientNormalize(c.LegalName)
	if c.DisplayName == "" {
		c.DisplayName = c.LegalName
	}
	c.Gstin = strings.ToUpper(c.Gstin)
	c.Pan = strings.ToUpper(c.Pan)
	c.Cin = strings.ToUpper(c.Cin)
	return nil
}

var clientUuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`)

func ClientCanonicalID(raw string) (string, error) {
	s := strings.TrimPrefix(strings.TrimSuffix(raw, "}"), "{")
	if strings.HasPrefix(strings.ToLower(s), "urn:uuid:") {
		s = s[9:]
	}
	if !clientUuidPattern.MatchString(s) {
		return "", ClientInvalid("Input should be a valid UUID")
	}
	s = strings.ToLower(strings.ReplaceAll(s, "-", ""))
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}

func clientWhitespace(r rune) bool      { return unicode.IsSpace(r) }
func ClientTrim(s string) string        { return strings.TrimFunc(s, clientWhitespace) }
func ClientHasWhitespace(s string) bool { return strings.ContainsFunc(s, clientWhitespace) }

// DirectoryEntry exposes client identity/status without contacts or financial identifiers.
type ClientDirectoryEntry struct {
	ID, Name, Code string
	Active         bool
}
