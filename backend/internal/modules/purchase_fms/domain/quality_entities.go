package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type QualityTimestamp = time.Time
type QualityError = Error

func QualityInvalid(f, m string) error { return &QualityError{422, f, m} }

var QualityUnauthorized = &QualityError{401, "", "Authentication required"}
var QualityForbidden = &QualityError{403, "", "Current quality permission and Step 14 work scope are required"}
var QualityMissing = &QualityError{404, "", "Quality case or current issued report not found"}
var QualityStale = &QualityError{409, "", "Quality or its report source changed; retain your draft, reload and review"}
var qualityUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func QualityID(s string) error {
	if !qualityUuid.MatchString(s) {
		return QualityInvalid("id", "Use a valid identifier")
	}
	return nil
}
func QualityText(f, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || required && s == "" {
		return s, QualityInvalid(f, "Provide a value within the field limit")
	}
	return s, nil
}
func QualityToday(t QualityTimestamp) string {
	return t.In(time.FixedZone("India", 19800)).Format("2006-01-02")
}
func QualityDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() > 0 && t.Format("2006-01-02") == s
}

type QualityAccess struct{ Read, Write, Verify, Scoped, Work bool }
type QualitySourceFacts struct {
	ProjectID         string `json:"project_id"`
	PackageID         string `json:"package_id"`
	ReportID          string `json:"report_id"`
	ReportVersion     int32  `json:"report_version"`
	ReportRevision    int32  `json:"report_revision"`
	IRNumber          string `json:"ir_number"`
	IRNNumber         string `json:"irn_number"`
	IssueDate         string `json:"issue_date"`
	Quantity          string `json:"cleared_quantity"`
	UOM               string `json:"uom"`
	OrderID           string `json:"order_id"`
	OrderVersion      int32  `json:"order_version"`
	Number            string `json:"number"`
	SupplierName      string `json:"supplier_name"`
	Description       string `json:"description"`
	InspectionID      string `json:"inspection_id"`
	InspectionVersion int32  `json:"inspection_version"`
}
type QualitySource struct {
	QualitySourceFacts
	Available bool     `json:"available"`
	Blockers  []string `json:"blockers"`
}
type QualityDetails struct {
	Number            string `json:"cert_number"`
	Issuer            string `json:"issued_by"`
	ValidUntil        string `json:"valid_until"`
	DocumentReference string `json:"file_name"`
}
type QualityCertificate struct {
	Code  string `json:"code"`
	Title string `json:"title"`
	QualityDetails
	Verified   bool              `json:"verified"`
	VerifiedAt *QualityTimestamp `json:"verified_at"`
	VerifiedBy string            `json:"verified_by_id"`
	Active     bool              `json:"is_active"`
	CreatedAt  QualityTimestamp  `json:"created_at"`
	UpdatedAt  QualityTimestamp  `json:"updated_at"`
	CreatedBy  string            `json:"created_by_id"`
	UpdatedBy  string            `json:"updated_by_id"`
}
type QualityRecord struct {
	ID           string               `json:"id"`
	ProjectID    string               `json:"project_id"`
	PackageID    string               `json:"package_id"`
	Version      int32                `json:"version"`
	Revision     int32                `json:"revision"`
	Source       QualitySource        `json:"source_snapshot"`
	Certificates []QualityCertificate `json:"certificates"`
	Active       bool                 `json:"is_active"`
	CreatedAt    QualityTimestamp     `json:"created_at"`
	UpdatedAt    QualityTimestamp     `json:"updated_at"`
	CreatedBy    string               `json:"created_by_id"`
	UpdatedBy    string               `json:"updated_by_id"`
}
type QualityCommand struct {
	Version int32           `json:"version"`
	Action  string          `json:"action"`
	Reason  string          `json:"reason"`
	Code    string          `json:"certificate_code,omitempty"`
	Details *QualityDetails `json:"details,omitempty"`
}
type QualityEvent struct {
	ID      string           `json:"id"`
	CaseID  string           `json:"case_id"`
	Action  string           `json:"action"`
	Code    string           `json:"certificate_code"`
	Reason  string           `json:"reason"`
	Before  *QualityRecord   `json:"before"`
	After   QualityRecord    `json:"after"`
	At      QualityTimestamp `json:"at"`
	ActorID string           `json:"actor_id"`
}
type QualityEligibility struct {
	Ready         bool     `json:"ready"`
	VerifiedCount int      `json:"verified_count"`
	RequiredCount int      `json:"required_count"`
	Blockers      []string `json:"blockers"`
}
type QualityWorkspace struct {
	Record             *QualityRecord      `json:"record"`
	Source             QualitySource       `json:"source"`
	Actions            []string            `json:"actions"`
	Eligibility        QualityEligibility  `json:"eligibility"`
	CertificateActions map[string][]string `json:"certificate_actions"`
	History            []QualityEvent      `json:"history"`
	TaskOnly           bool                `json:"task_only"`
}

var qualityDefinitions = []struct{ code, title string }{{"TC", "Type Test Certificate"}, {"TR", "Routine Test Report"}, {"MTC", "Material Test Certificate"}, {"RTC", "Routine / Galvanizing Certificate"}, {"Warranty", "Warranty / Guarantee Undertaking"}}

func qualityCode(s string) bool {
	for _, d := range qualityDefinitions {
		if d.code == s {
			return true
		}
	}
	return false
}
func QualityDefaults(a Principal, at QualityTimestamp) []QualityCertificate {
	out := []QualityCertificate{}
	for _, d := range qualityDefinitions {
		out = append(out, QualityCertificate{Code: d.code, Title: d.title, Active: true, CreatedAt: at, UpdatedAt: at, CreatedBy: a.ID, UpdatedBy: a.ID})
	}
	return out
}
func QualityCopy(r QualityRecord) QualityRecord {
	r.Source.Blockers = append([]string{}, r.Source.Blockers...)
	r.Certificates = append([]QualityCertificate{}, r.Certificates...)
	for i, c := range r.Certificates {
		if c.VerifiedAt != nil {
			at := *c.VerifiedAt
			r.Certificates[i].VerifiedAt = &at
		}
	}
	return r
}
