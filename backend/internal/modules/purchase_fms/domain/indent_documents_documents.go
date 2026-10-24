package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type IndentDocumentsError = Error

func IndentDocumentsInvalid(f, m string) error { return &IndentDocumentsError{422, f, m} }

var IndentDocumentsUnauthorized = &IndentDocumentsError{401, "", "Invalid or expired credentials"}
var IndentDocumentsForbidden = &IndentDocumentsError{403, "", "Vendor indent permission or project scope denied"}
var IndentDocumentsMissing = &IndentDocumentsError{404, "", "Vendor indent not found in this BO"}
var IndentDocumentsStale = &IndentDocumentsError{409, "", "Vendor indent or groups changed; reload before saving"}

type IndentDocumentsTimestamp = time.Time

var indentDocumentsUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func IndentDocumentsID(s string) (string, error) {
	s = strings.ToLower(s)
	if !indentDocumentsUuid.MatchString(s) {
		return s, IndentDocumentsInvalid("id", "Use a valid record identifier")
	}
	return s, nil
}

type IndentDocumentsItem struct {
	ID             string `json:"id"`
	SKU            string `json:"sku"`
	Description    string `json:"description"`
	EngineeringKey string `json:"engineering_key"`
	UOM            string `json:"uom"`
	Manufacturer   string `json:"manufacturer"`
}
type IndentDocumentsLine struct {
	ID           string              `json:"id"`
	ItemID       *string             `json:"item_id"`
	Quantity     *string             `json:"quantity"`
	RequiredDate *string             `json:"required_date"`
	Reference    string              `json:"reference"`
	Notes        string              `json:"notes"`
	Snapshot     IndentDocumentsItem `json:"item_snapshot"`
	DemandID     *string             `json:"demand_id"`
}
type IndentDocumentsVendor struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Code    string         `json:"code"`
	Active  bool           `json:"is_active"`
	Data    map[string]any `json:"data"`
	Version int32          `json:"version"`
}
type IndentDocumentsPrintRequest struct {
	RequestedAt time.Time `json:"requested_at"`
	ActorID     string    `json:"actor_id"`
}
type IndentDocumentsData struct {
	Terms             *IndentDocumentsTerms         `json:"terms,omitempty"`
	GeneratedByName   string                        `json:"generated_by_name"`
	ProjectName       string                        `json:"project_name"`
	GroupName         string                        `json:"group_name"`
	ScopeRevision     int32                         `json:"scope_revision"`
	Vendor            IndentDocumentsVendor         `json:"vendor"`
	Lines             []IndentDocumentsLine         `json:"lines"`
	Number            string                        `json:"number"`
	GeneratedAt       time.Time                     `json:"generated_at"`
	GeneratedByID     string                        `json:"generated_by_id"`
	PrintRequests     []IndentDocumentsPrintRequest `json:"print_requests"`
	SentDate          *string                       `json:"sent_date"`
	Channel           string                        `json:"channel"`
	DispatchReference string                        `json:"dispatch_reference"`
	SentRecordedAt    *time.Time                    `json:"sent_recorded_at"`
	SentByID          *string                       `json:"sent_by_id"`
	IsCurrent         bool                          `json:"is_current"`
}
type IndentDocumentsDocument struct {
	ID        string              `json:"id"`
	ProjectID string              `json:"project_id"`
	BOID      string              `json:"bo_id"`
	GroupID   string              `json:"group_id"`
	VendorID  string              `json:"vendor_id"`
	Version   int32               `json:"version"`
	Data      IndentDocumentsData `json:"data"`
	Active    bool                `json:"is_active"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
	CreatedBy *string             `json:"created_by_id"`
	UpdatedBy *string             `json:"updated_by_id"`
}
type IndentDocumentsCommand struct {
	Action            string  `json:"action"`
	GroupID           *string `json:"group_id"`
	VendorID          *string `json:"vendor_id"`
	DocumentID        *string `json:"document_id"`
	Version           int32   `json:"version"`
	GroupVersion      int32   `json:"group_version"`
	BOVersion         int32   `json:"bo_version"`
	SentDate          *string `json:"sent_date"`
	Channel           string  `json:"channel"`
	DispatchReference string  `json:"dispatch_reference"`
}

func (c IndentDocumentsCommand) Validate() (IndentDocumentsCommand, error) {
	if c.Version < 0 || c.GroupVersion < 0 || c.BOVersion < 0 {
		return c, IndentDocumentsInvalid("version", "Use current saved versions")
	}
	for _, id := range []*string{c.GroupID, c.VendorID, c.DocumentID} {
		if id != nil {
			v, e := IndentDocumentsID(*id)
			if e != nil {
				return c, e
			}
			*id = v
		}
	}
	switch c.Action {
	case "generate":
		if c.GroupID == nil || c.VendorID == nil {
			return c, IndentDocumentsInvalid("group_id", "Select group and vendor")
		}
	case "print", "send":
		if c.DocumentID == nil {
			return c, IndentDocumentsInvalid("document_id", "Select a saved vendor indent")
		}
	default:
		return c, IndentDocumentsInvalid("action", "Unknown indent action")
	}
	c.Channel = strings.TrimSpace(c.Channel)
	c.DispatchReference = strings.TrimSpace(c.DispatchReference)
	if utf8.RuneCountInString(c.Channel) > 80 || utf8.RuneCountInString(c.DispatchReference) > 500 {
		return c, IndentDocumentsInvalid("dispatch_reference", "Dispatch details exceed permitted length")
	}
	if c.Action == "send" && (c.SentDate == nil || !IndentDocumentsDate(*c.SentDate) || c.Channel == "" || c.DispatchReference == "") {
		return c, IndentDocumentsInvalid("sent_date", "Record a valid actual sent date, channel and reference")
	}
	return c, nil
}
func IndentDocumentsDate(s string) bool {
	v, e := time.Parse("2006-01-02", s)
	return e == nil && v.Year() > 0 && v.Year() <= 9999 && v.Format("2006-01-02") == s
}
func IndentDocumentsLocal(t time.Time) time.Time { return t.In(time.FixedZone("Asia/Kolkata", 19800)) }
func IndentDocumentsNumber(id string) string {
	return "IND-" + strings.ToUpper(strings.ReplaceAll(id, "-", ""))
}
func IndentDocumentsValidateDispatch(d IndentDocumentsData, date string, now time.Time) error {
	if !IndentDocumentsDate(date) || date < IndentDocumentsLocal(d.GeneratedAt).Format("2006-01-02") || date > IndentDocumentsLocal(now).Format("2006-01-02") {
		return IndentDocumentsInvalid("sent_date", "Sent date must be between generation date and today")
	}
	if d.SentDate != nil {
		return IndentDocumentsInvalid("sent_date", "Dispatch is already recorded")
	}
	return nil
}
func IndentDocumentsCopy(d IndentDocumentsDocument) IndentDocumentsDocument {
	if d.Data.Terms != nil {
		terms := *d.Data.Terms
		terms.Notes = append([]string{}, terms.Notes...)
		terms.Conditions = append([]string{}, terms.Conditions...)
		d.Data.Terms = &terms
	}
	d.Data.PrintRequests = append([]IndentDocumentsPrintRequest{}, d.Data.PrintRequests...)
	d.Data.Lines = append([]IndentDocumentsLine{}, d.Data.Lines...)
	return d
}

type IndentDocumentsScope struct {
	Data     IndentDocumentsData
	Revision int32
}
type IndentDocumentsRecipients struct {
	IDs             []string
	ManagerAssigned bool
}
type IndentDocumentsEvent struct {
	ID         string                   `json:"id"`
	DocumentID string                   `json:"document_id"`
	Action     string                   `json:"action"`
	Reason     string                   `json:"reason"`
	Before     *IndentDocumentsDocument `json:"before"`
	After      IndentDocumentsDocument  `json:"after"`
	ActorID    string                   `json:"created_by_id"`
	CreatedAt  time.Time                `json:"created_at"`
}
type IndentDocumentsWorkspace struct {
	Documents       []IndentDocumentsDocument `json:"documents"`
	History         []IndentDocumentsEvent    `json:"history"`
	ManagerAssigned bool                      `json:"manager_assigned"`
	CanGenerate     bool                      `json:"can_generate"`
	CanPrint        bool                      `json:"can_print"`
	CanDispatch     bool                      `json:"can_dispatch"`
}
type IndentDocumentsNotice struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	BOID      string    `json:"bo_id"`
	Number    string    `json:"number"`
	Group     string    `json:"group"`
	Vendor    string    `json:"vendor"`
	CreatedAt time.Time `json:"created_at"`
}
