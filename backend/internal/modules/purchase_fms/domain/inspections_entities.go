package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type InspectionsTimestamp = time.Time
type InspectionsError = Error

func InspectionsInvalid(f, m string) error { return &InspectionsError{422, f, m} }

var InspectionsUnauthorized = &InspectionsError{401, "", "Authentication required"}
var InspectionsForbidden = &InspectionsError{403, "", "Current inspection permission and stage12 scope are required"}
var InspectionsMissing = &InspectionsError{404, "", "Inspection or its approved source not found"}
var InspectionsStale = &InspectionsError{409, "", "Inspection or its approved source changed; retain your draft, reload and review"}
var inspectionsUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func InspectionsID(s string) error {
	if !inspectionsUuid.MatchString(s) {
		return InspectionsInvalid("id", "Use a valid identifier")
	}
	return nil
}
func InspectionsText(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || (required && s == "") {
		return s, InspectionsInvalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func InspectionsToday(t time.Time) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}
func InspectionsDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type InspectionsAccess struct{ Read, Write, Scoped, Work bool }

type InspectionsSourceFacts struct {
	ProjectID            string `json:"project_id"`
	PackageID            string `json:"package_id"`
	OrderID              string `json:"order_id"`
	OrderVersion         int32  `json:"order_version"`
	Number               string `json:"number"`
	Date                 string `json:"date"`
	SupplierID           string `json:"supplier_id"`
	SupplierName         string `json:"supplier_name"`
	Description          string `json:"description"`
	Quantity             string `json:"quantity"`
	UOM                  string `json:"uom"`
	AcceptanceID         string `json:"acceptance_id"`
	AcceptanceVersion    int32  `json:"acceptance_version"`
	TechnicalID          string `json:"technical_id"`
	TechnicalVersion     int32  `json:"technical_version"`
	TechnicalRevision    int32  `json:"technical_revision"`
	QAPReference         string `json:"qap_reference"`
	QAPDate              string `json:"qap_date"`
	QAPFileID            string `json:"qap_file_id"`
	SignedApprovalFileID string `json:"signed_approval_file_id"`
}
type InspectionsSource struct {
	InspectionsSourceFacts
	Available bool     `json:"available"`
	Blockers  []string `json:"blockers"`
}
type InspectionsWitnessPoint struct {
	Code      string `json:"code"`
	Title     string `json:"title"`
	Mandatory bool   `json:"is_witness"`
	Cleared   bool   `json:"is_cleared"`
	Remarks   string `json:"remarks"`
}
type InspectionsInspection struct {
	Status         string                    `json:"status"`
	CallReference  string                    `json:"call_notice_ref"`
	CallDate       string                    `json:"call_date"`
	ScheduledDate  string                    `json:"scheduled_date"`
	ActualDate     string                    `json:"actual_date"`
	Agency         string                    `json:"inspection_agency"`
	Inspector      string                    `json:"inspector_name"`
	Factory        string                    `json:"factory_location"`
	Points         []InspectionsWitnessPoint `json:"witness_points"`
	NoPointsReason string                    `json:"no_witness_points_reason"`
	Findings       string                    `json:"findings"`
}
type InspectionsRecord struct {
	ID         string                `json:"id"`
	ProjectID  string                `json:"project_id"`
	PackageID  string                `json:"package_id"`
	Version    int32                 `json:"version"`
	Revision   int32                 `json:"revision"`
	Source     InspectionsSource     `json:"source_snapshot"`
	Inspection InspectionsInspection `json:"inspection"`
	Active     bool                  `json:"is_active"`
	CreatedAt  InspectionsTimestamp  `json:"created_at"`
	UpdatedAt  InspectionsTimestamp  `json:"updated_at"`
	CreatedBy  string                `json:"created_by_id"`
	UpdatedBy  string                `json:"updated_by_id"`
}
type InspectionsCommand struct {
	Version    int32                       `json:"version"`
	Action     string                      `json:"action"`
	Reason     string                      `json:"reason"`
	Inspection *InspectionsInspectionInput `json:"inspection,omitempty"`
}
type InspectionsWitnessInput struct {
	Code      string `json:"code"`
	Title     string `json:"title"`
	Mandatory *bool  `json:"is_witness"`
	Cleared   *bool  `json:"is_cleared"`
	Remarks   string `json:"remarks"`
}
type InspectionsInspectionInput struct {
	Status         string                    `json:"status"`
	CallReference  string                    `json:"call_notice_ref"`
	CallDate       string                    `json:"call_date"`
	ScheduledDate  string                    `json:"scheduled_date"`
	ActualDate     string                    `json:"actual_date"`
	Agency         string                    `json:"inspection_agency"`
	Inspector      string                    `json:"inspector_name"`
	Factory        string                    `json:"factory_location"`
	Points         []InspectionsWitnessInput `json:"witness_points"`
	NoPointsReason string                    `json:"no_witness_points_reason"`
	Findings       string                    `json:"findings"`
}
type InspectionsEvent struct {
	ID           string               `json:"id"`
	InspectionID string               `json:"inspection_id"`
	Action       string               `json:"action"`
	Reason       string               `json:"reason"`
	Before       *InspectionsRecord   `json:"before"`
	After        InspectionsRecord    `json:"after"`
	At           InspectionsTimestamp `json:"at"`
	ActorID      string               `json:"actor_id"`
}
type InspectionsWorkspace struct {
	Record   *InspectionsRecord `json:"record"`
	Source   InspectionsSource  `json:"source"`
	Actions  []string           `json:"actions"`
	History  []InspectionsEvent `json:"history"`
	TaskOnly bool               `json:"task_only"`
}

func InspectionsCopy(r InspectionsRecord) InspectionsRecord {
	r.Source.Blockers = append([]string{}, r.Source.Blockers...)
	r.Inspection.Points = append([]InspectionsWitnessPoint{}, r.Inspection.Points...)
	return r
}
