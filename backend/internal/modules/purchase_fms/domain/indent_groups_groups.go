package domain

import (
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type IndentGroupsError = Error

func IndentGroupsInvalid(f, m string) error { return &IndentGroupsError{422, f, m} }

var IndentGroupsUnauthorized = &IndentGroupsError{401, "", "Invalid or expired credentials"}
var IndentGroupsForbidden = &IndentGroupsError{403, "", "Purchase group permission or project scope denied"}
var IndentGroupsMissing = &IndentGroupsError{404, "", "Purchase group not found"}
var IndentGroupsStale = &IndentGroupsError{409, "", "Purchase group or linked material requirement changed; reload the saved version"}

var indentGroupsUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func IndentGroupsID(s string) (string, error) {
	s = strings.ToLower(s)
	if !indentGroupsUuid.MatchString(s) {
		return s, IndentGroupsInvalid("id", "Use a valid group record identifier")
	}
	return s, nil
}

type IndentGroupsTimestamp = time.Time
type IndentGroupsItem struct {
	ID             string `json:"id"`
	SKU            string `json:"sku"`
	Description    string `json:"description"`
	EngineeringKey string `json:"engineering_key"`
	UOM            string `json:"uom"`
	Manufacturer   string `json:"manufacturer"`
}
type IndentGroupsLine struct {
	ID           string           `json:"id"`
	ItemID       *string          `json:"item_id"`
	Quantity     *string          `json:"quantity"`
	RequiredDate *string          `json:"required_date"`
	Reference    string           `json:"reference"`
	Notes        string           `json:"notes"`
	Snapshot     IndentGroupsItem `json:"item_snapshot"`
	DemandID     *string          `json:"demand_id"`
}
type IndentGroupsVendor struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Code    string         `json:"code"`
	Active  bool           `json:"is_active"`
	Data    map[string]any `json:"data"`
	Version int32          `json:"version"`
}
type IndentGroupsInvitation struct {
	VendorID          string    `json:"vendor_id"`
	ScopeRevision     int32     `json:"scope_revision"`
	LineIDs           []string  `json:"line_ids"`
	SentDate          string    `json:"sent_date"`
	Channel           string    `json:"channel"`
	DispatchReference string    `json:"dispatch_reference"`
	RecordedByID      string    `json:"recorded_by_id"`
	RecordedAt        time.Time `json:"recorded_at"`
}
type IndentGroupsGroupInput struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	LineIDs   []string `json:"line_ids"`
	VendorIDs []string `json:"vendor_ids"`
}
type IndentGroupsGroup struct {
	IndentGroupsGroupInput
	Invitations   []IndentGroupsInvitation `json:"invitations"`
	ScopeRevision int32                    `json:"scope_revision"`
	NameKey       string                   `json:"-"`
}
type IndentGroupsRecord struct {
	BOID            string               `json:"bo_id"`
	ProjectID       string               `json:"project_id"`
	Version         int32                `json:"version"`
	State           string               `json:"state"`
	BOVersion       int32                `json:"bo_version"`
	Groups          []IndentGroupsGroup  `json:"groups"`
	SourceLines     []IndentGroupsLine   `json:"source_lines"`
	Number          string               `json:"number"`
	SourceVendors   []IndentGroupsVendor `json:"source_vendors"`
	SelectedLineIDs []string             `json:"selected_line_ids"`
	Active          bool                 `json:"is_active"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	CreatedBy       *string              `json:"created_by_id"`
	UpdatedBy       *string              `json:"updated_by_id"`
}
type IndentGroupsCommand struct {
	InvitationScopeRevision *int32                   `json:"-"`
	InvitationLineIDs       []string                 `json:"-"`
	Version                 int32                    `json:"version"`
	BOVersion               int32                    `json:"bo_version"`
	Action                  string                   `json:"action"`
	Reason                  string                   `json:"reason"`
	Groups                  []IndentGroupsGroupInput `json:"groups"`
	GroupID                 *string                  `json:"group_id"`
	LineID                  *string                  `json:"line_id"`
	LineIDs                 []string                 `json:"line_ids"`
	VendorIDs               []string                 `json:"vendor_ids"`
	SelectedLineIDs         []string                 `json:"selected_line_ids"`
	VendorID                *string                  `json:"vendor_id"`
	SentDate                *string                  `json:"sent_date"`
	Channel                 string                   `json:"channel"`
	DispatchReference       string                   `json:"dispatch_reference"`
}
type IndentGroupsSource struct {
	Version   int32
	State     string
	Lines     []IndentGroupsLine
	Available bool
}
type IndentGroupsEvent struct {
	ID        string              `json:"id"`
	BOID      string              `json:"bo_id"`
	Action    string              `json:"action"`
	Reason    string              `json:"reason"`
	Before    *IndentGroupsRecord `json:"before"`
	After     IndentGroupsRecord  `json:"after"`
	CreatedAt time.Time           `json:"created_at"`
	ActorID   string              `json:"created_by_id"`
}
type IndentGroupsNamedVendor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type IndentGroupsReportInvitation struct {
	IndentGroupsInvitation
	VendorName string `json:"vendor_name"`
}
type IndentGroupsReportRow struct {
	ID              string                         `json:"id"`
	SKU             string                         `json:"sku"`
	Item            string                         `json:"item"`
	Quantity        *string                        `json:"quantity"`
	UOM             string                         `json:"uom"`
	Group           string                         `json:"group"`
	SelectedVendors []IndentGroupsNamedVendor      `json:"selected_vendors"`
	Invitations     []IndentGroupsReportInvitation `json:"invitations"`
}
type IndentGroupsWorkspace struct {
	Record           *IndentGroupsRecord     `json:"record"`
	Vendors          []IndentGroupsVendor    `json:"vendors"`
	CanWrite         bool                    `json:"can_write"`
	CanManageVendors bool                    `json:"can_manage_vendors"`
	History          []IndentGroupsEvent     `json:"history"`
	ItemReport       []IndentGroupsReportRow `json:"item_report"`
}

func IndentGroupsReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > 2000 {
		return s, IndentGroupsInvalid("reason", "Record a reason within 2000 characters")
	}
	return s, nil
}
func IndentGroupsListIDs(ids []string, max int) ([]string, error) {
	if len(ids) > max {
		return ids, IndentGroupsInvalid("", "Too many selected records")
	}
	for i, id := range ids {
		v, e := IndentGroupsID(id)
		if e != nil {
			return ids, e
		}
		ids[i] = v
	}
	return ids, nil
}
func (c IndentGroupsCommand) Validate() (IndentGroupsCommand, error) {
	var e error
	c.Reason, e = IndentGroupsReason(c.Reason)
	if e != nil {
		return c, e
	}
	if c.Version < 0 || c.BOVersion < 0 {
		return c, IndentGroupsInvalid("version", "Use the current saved versions")
	}
	switch c.Action {
	case "save", "create", "add_vendors", "select_items", "save_groups", "set_vendors", "invite", "create_group", "add_items", "reset_item":
	default:
		return c, IndentGroupsInvalid("action", "Unknown group action")
	}
	if len(c.Groups) > 100 {
		return c, IndentGroupsInvalid("groups", "Use at most 100 item groups")
	}
	for i, g := range c.Groups {
		g.ID, e = IndentGroupsID(g.ID)
		if e != nil {
			return c, e
		}
		g.Name = strings.TrimSpace(g.Name)
		if g.Name == "" || utf8.RuneCountInString(g.Name) > 128 {
			return c, IndentGroupsInvalid("name", "Group name must contain 1–128 characters")
		}
		if len(g.LineIDs) == 0 {
			return c, IndentGroupsInvalid("line_ids", "Each group needs items")
		}
		g.LineIDs, e = IndentGroupsListIDs(g.LineIDs, 100)
		if e != nil {
			return c, e
		}
		g.VendorIDs, e = IndentGroupsListIDs(g.VendorIDs, 50)
		if e != nil {
			return c, e
		}
		c.Groups[i] = g
	}
	for _, pair := range []struct{ v *string }{{c.GroupID}, {c.LineID}, {c.VendorID}} {
		if pair.v != nil {
			v, e := IndentGroupsID(*pair.v)
			if e != nil {
				return c, e
			}
			*pair.v = v
		}
	}
	c.LineIDs, e = IndentGroupsListIDs(c.LineIDs, 100)
	if e != nil {
		return c, e
	}
	c.SelectedLineIDs, e = IndentGroupsListIDs(c.SelectedLineIDs, 100)
	if e != nil {
		return c, e
	}
	c.VendorIDs, e = IndentGroupsListIDs(c.VendorIDs, 50)
	if e != nil {
		return c, e
	}
	c.Channel = strings.TrimSpace(c.Channel)
	c.DispatchReference = strings.TrimSpace(c.DispatchReference)
	if utf8.RuneCountInString(c.Channel) > 80 || utf8.RuneCountInString(c.DispatchReference) > 500 {
		return c, IndentGroupsInvalid("dispatch_reference", "Dispatch details exceed the permitted length")
	}
	if c.SentDate != nil && !IndentGroupsDate(*c.SentDate) {
		return c, IndentGroupsInvalid("sent_date", "Use a valid dispatch date")
	}
	return c, nil
}
func IndentGroupsDate(s string) bool {
	v, e := time.Parse("2006-01-02", s)
	return e == nil && v.Year() > 0 && v.Year() <= 9999 && v.Format("2006-01-02") == s
}
func IndentGroupsDistinct(ids []string) bool {
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
func IndentGroupsContains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
func IndentGroupsSubset(ids, allowed []string) bool {
	for _, id := range ids {
		if !IndentGroupsContains(allowed, id) {
			return false
		}
	}
	return true
}
func IndentGroupsSameSet(a, b []string) bool {
	return len(a) == len(b) && IndentGroupsSubset(a, b) && IndentGroupsSubset(b, a)
}
func IndentGroupsValidateGroups(groups []IndentGroupsGroup, lines, vendors []string, complete bool) error {
	ids, names := map[string]bool{}, map[string]bool{}
	assigned := []string{}
	for _, g := range groups {
		if ids[g.ID] {
			return IndentGroupsInvalid("groups", "Group identifiers must be distinct")
		}
		ids[g.ID] = true
		if g.NameKey == "" || names[g.NameKey] {
			return IndentGroupsInvalid("name", "Give every group a distinct name")
		}
		names[g.NameKey] = true
		if len(g.LineIDs) == 0 || !IndentGroupsSubset(g.LineIDs, lines) {
			return IndentGroupsInvalid("line_ids", "Each group needs items from the current BO list")
		}
		assigned = append(assigned, g.LineIDs...)
		if !IndentGroupsDistinct(g.VendorIDs) || !IndentGroupsSubset(g.VendorIDs, vendors) {
			return IndentGroupsInvalid("vendor_ids", "Select distinct active vendors")
		}
		if complete && len(g.VendorIDs) < 3 {
			return IndentGroupsInvalid("vendor_ids", "Group "+g.Name+" needs at least three active vendors")
		}
	}
	if !IndentGroupsDistinct(assigned) {
		return IndentGroupsInvalid("line_ids", "An item may belong to only one group")
	}
	if complete && (len(groups) == 0 || !IndentGroupsSameSet(assigned, lines)) {
		return IndentGroupsInvalid("groups", "Assign every BO item to a group before creating indents")
	}
	return nil
}
func IndentGroupsLineIDs(lines []IndentGroupsLine) []string {
	out := []string{}
	for _, l := range lines {
		out = append(out, l.ID)
	}
	return out
}
func IndentGroupsVendorIDs(vendors []IndentGroupsVendor) []string {
	out := []string{}
	for _, v := range vendors {
		out = append(out, v.ID)
	}
	return out
}
func IndentGroupsGroupLines(groups []IndentGroupsGroup) []string {
	out := []string{}
	for _, g := range groups {
		out = append(out, g.LineIDs...)
	}
	return out
}
func IndentGroupsGroupVendors(groups []IndentGroupsGroup) []string {
	out := []string{}
	for _, g := range groups {
		out = append(out, g.VendorIDs...)
	}
	return out
}
func IndentGroupsNumber(bo string) string {
	return "GI-" + strings.ToUpper(strings.ReplaceAll(bo, "-", "")[:12])
}
func IndentGroupsCandidates(record *IndentGroupsRecord, current []IndentGroupsVendor) []IndentGroupsVendor {
	out := append([]IndentGroupsVendor{}, current...)
	if record == nil {
		return out
	}
	active := IndentGroupsVendorIDs(current)
	selected := IndentGroupsGroupVendors(record.Groups)
	sort.Strings(selected)
	seen := map[string]bool{}
	for _, id := range selected {
		if IndentGroupsContains(active, id) || seen[id] {
			continue
		}
		seen[id] = true
		v := IndentGroupsVendor{ID: id, Name: "Unavailable vendor", Data: map[string]any{}}
		for _, s := range record.SourceVendors {
			if s.ID == id {
				v = s
			}
		}
		v.Active = false
		out = append(out, v)
	}
	return out
}
func IndentGroupsReport(record *IndentGroupsRecord, lines []IndentGroupsLine, vendors []IndentGroupsVendor) []IndentGroupsReportRow {
	out := []IndentGroupsReportRow{}
	if record == nil {
		return out
	}
	if record.State == "created" {
		lines = record.SourceLines
	}
	name := func(id string) string {
		for _, v := range vendors {
			if v.ID == id {
				return v.Name
			}
		}
		return "Unavailable vendor"
	}
	for _, line := range lines {
		if !IndentGroupsContains(record.SelectedLineIDs, line.ID) {
			continue
		}
		row := IndentGroupsReportRow{ID: line.ID, SKU: line.Snapshot.SKU, Item: line.Snapshot.Description, Quantity: line.Quantity, UOM: line.Snapshot.UOM, Group: "Ungrouped", SelectedVendors: []IndentGroupsNamedVendor{}, Invitations: []IndentGroupsReportInvitation{}}
		for _, g := range record.Groups {
			if !IndentGroupsContains(g.LineIDs, line.ID) {
				continue
			}
			row.Group = g.Name
			for _, id := range g.VendorIDs {
				row.SelectedVendors = append(row.SelectedVendors, IndentGroupsNamedVendor{id, name(id)})
			}
			for _, i := range g.Invitations {
				if IndentGroupsContains(i.LineIDs, line.ID) {
					row.Invitations = append(row.Invitations, IndentGroupsReportInvitation{i, name(i.VendorID)})
				}
			}
			break
		}
		out = append(out, row)
	}
	return out
}
func IndentGroupsCopy(r IndentGroupsRecord) IndentGroupsRecord {
	r.Groups = append([]IndentGroupsGroup{}, r.Groups...)
	for i, g := range r.Groups {
		g.LineIDs = append([]string{}, g.LineIDs...)
		g.VendorIDs = append([]string{}, g.VendorIDs...)
		g.Invitations = append([]IndentGroupsInvitation{}, g.Invitations...)
		for n := range g.Invitations {
			g.Invitations[n].LineIDs = append([]string{}, g.Invitations[n].LineIDs...)
		}
		r.Groups[i] = g
	}
	r.SourceLines = append([]IndentGroupsLine{}, r.SourceLines...)
	r.SourceVendors = append([]IndentGroupsVendor{}, r.SourceVendors...)
	r.SelectedLineIDs = append([]string{}, r.SelectedLineIDs...)
	return r
}
func IndentGroupsOrderedEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if b[i] != v {
			return false
		}
	}
	return true
}
func IndentGroupsToday(now time.Time) string {
	return now.In(time.FixedZone("Asia/Kolkata", 19800)).Format("2006-01-02")
}
