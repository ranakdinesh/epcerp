package domain

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Capture struct {
	PurchaseEnabled       bool
	ActualEndDate         string
	Calculation           Calculation
	ShortName             string
	ProjectType           string
	SiteName              string
	SiteState             string
	SiteCity              string
	ScopeSummary          string
	OperationsManagerID   string
	ContractValue         string
	Currency              string
	LOINumber             string
	PONumber              string
	ScheduleStartBasis    string
	PlannedStartDate      string
	PlannedCompletionDate string
	LOIDate               string
	PODate                string
	ActualStartDate       string
	SiteHandoverDate      string
	PlannedDurationDays   string
	SafetyBufferPercent   string
	SafetyBufferDays      string
	Remarks               string
}

var amountPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,13})(\.[0-9]{1,2})?$`)
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

func (c Capture) Validate(name string) (Capture, error) {
	return c.validate(name, true)
}

// ValidateDraft validates entered values while allowing unfinished draft fields.
func (c Capture) ValidateDraft() (Capture, error) {
	return c.validate("", false)
}

func (c Capture) validate(name string, complete bool) (Capture, error) {
	c.ShortName = strings.TrimSpace(c.ShortName)
	if utf8.RuneCountInString(c.ShortName) > 255 {
		return c, &Invalid{"short_name", "Value exceeds permitted length"}
	}
	c.ProjectType = strings.TrimSpace(c.ProjectType)
	if utf8.RuneCountInString(c.ProjectType) > 32 {
		return c, &Invalid{"project_type", "Value exceeds permitted length"}
	}
	c.SiteName = strings.TrimSpace(c.SiteName)
	if utf8.RuneCountInString(c.SiteName) > 255 {
		return c, &Invalid{"site_name", "Value exceeds permitted length"}
	}
	c.SiteState = strings.TrimSpace(c.SiteState)
	if utf8.RuneCountInString(c.SiteState) > 128 {
		return c, &Invalid{"site_state", "Value exceeds permitted length"}
	}
	c.SiteCity = strings.TrimSpace(c.SiteCity)
	if utf8.RuneCountInString(c.SiteCity) > 128 {
		return c, &Invalid{"site_city", "Value exceeds permitted length"}
	}
	c.ScopeSummary = strings.TrimSpace(c.ScopeSummary)
	if utf8.RuneCountInString(c.ScopeSummary) > 10000 {
		return c, &Invalid{"scope_summary", "Value exceeds permitted length"}
	}
	c.Currency = strings.TrimSpace(c.Currency)
	if utf8.RuneCountInString(c.Currency) > 3 {
		return c, &Invalid{"currency", "Value exceeds permitted length"}
	}
	c.LOINumber = strings.TrimSpace(c.LOINumber)
	if utf8.RuneCountInString(c.LOINumber) > 64 {
		return c, &Invalid{"loi_number", "Value exceeds permitted length"}
	}
	c.PONumber = strings.TrimSpace(c.PONumber)
	if utf8.RuneCountInString(c.PONumber) > 64 {
		return c, &Invalid{"po_number", "Value exceeds permitted length"}
	}
	c.ScheduleStartBasis = strings.TrimSpace(c.ScheduleStartBasis)
	if utf8.RuneCountInString(c.ScheduleStartBasis) > 32 {
		return c, &Invalid{"schedule_start_basis", "Value exceeds permitted length"}
	}
	c.Remarks = strings.TrimSpace(c.Remarks)
	if utf8.RuneCountInString(c.Remarks) > 10000 {
		return c, &Invalid{"remarks", "Value exceeds permitted length"}
	}
	if c.ShortName == "" {
		c.ShortName = name
	}
	if c.Currency == "" {
		c.Currency = "INR"
	}
	c.Currency = strings.ToUpper(c.Currency)
	if !currencyPattern.MatchString(c.Currency) {
		return c, &Invalid{"currency", "Use a three-letter currency code"}
	}
	if (complete || c.ContractValue != "") && !amountPattern.MatchString(c.ContractValue) {
		return c, &Invalid{"contract_value", "Enter a nonnegative project value with at most 14 integer and 2 decimal digits"}
	}
	if (complete || c.ProjectType != "") && c.ProjectType != "electrical" && c.ProjectType != "solar" && c.ProjectType != "ev" && c.ProjectType != "mixed" {
		return c, &Invalid{"project_type", "Select a business type"}
	}
	if complete && c.SiteState == "" {
		return c, &Invalid{"site_state", "Select the site state"}
	}
	if complete && c.SiteCity == "" {
		return c, &Invalid{"site_city", "Select the city / location"}
	}
	var err error
	if c.OperationsManagerID != "" {
		if c.OperationsManagerID, err = ID(c.OperationsManagerID); err != nil {
			return c, &Invalid{"operations_manager_id", "Select an active operations manager"}
		}
	}
	return c.validateSchedule(complete)
}
func (c Capture) ValidateSchedule() (Capture, error) {
	return c.validateSchedule(true)
}

func (c Capture) validateSchedule(complete bool) (Capture, error) {
	if (c.PlannedDurationDays == "" || c.ActualStartDate != "") && c.PlannedCompletionDate != "" {
		c.PlannedDurationDays = "1"
	}
	duration, err := strconv.Atoi(c.PlannedDurationDays)
	if (complete || c.PlannedDurationDays != "") && (err != nil || duration < 1 || duration > 36500) {
		return c, &Invalid{"planned_duration_days", "Enter completion days from 1 to 36500"}
	}
	if c.PlannedDurationDays != "" {
		c.PlannedDurationDays = strconv.Itoa(duration)
	}
	if c.SafetyBufferDays != "" {
		v, e := strconv.Atoi(c.SafetyBufferDays)
		if e != nil || v < 0 || v > 36500 {
			return c, &Invalid{"safety_buffer_days", "Enter buffer days from 0 to 36500"}
		}
		c.SafetyBufferDays = strconv.Itoa(v)
	}
	if c.SafetyBufferPercent != "" {
		if !amountPattern.MatchString(c.SafetyBufferPercent) {
			return c, &Invalid{"safety_buffer_percent", "Enter a percentage with at most 2 decimal places"}
		}
		v, _ := strconv.ParseFloat(c.SafetyBufferPercent, 64)
		if v > 100 {
			return c, &Invalid{"safety_buffer_percent", "Enter a percentage from 0 to 100"}
		}
	}
	if c.PlannedStartDate != "" {
		v, e := time.Parse("2006-01-02", c.PlannedStartDate)
		if e != nil || v.Year() < 1 || v.Year() > 9999 {
			return c, &Invalid{"planned_start_date", "Use a valid date in YYYY-MM-DD format"}
		}
	}
	if c.PlannedCompletionDate != "" {
		v, e := time.Parse("2006-01-02", c.PlannedCompletionDate)
		if e != nil || v.Year() < 1 || v.Year() > 9999 {
			return c, &Invalid{"planned_completion_date", "Use a valid date in YYYY-MM-DD format"}
		}
	}
	if c.LOIDate != "" {
		v, e := time.Parse("2006-01-02", c.LOIDate)
		if e != nil || v.Year() < 1 || v.Year() > 9999 {
			return c, &Invalid{"loi_date", "Use a valid date in YYYY-MM-DD format"}
		}
	}
	if c.PODate != "" {
		v, e := time.Parse("2006-01-02", c.PODate)
		if e != nil || v.Year() < 1 || v.Year() > 9999 {
			return c, &Invalid{"po_date", "Use a valid date in YYYY-MM-DD format"}
		}
	}
	if c.ActualStartDate != "" {
		v, e := time.Parse("2006-01-02", c.ActualStartDate)
		if e != nil || v.Year() < 1 || v.Year() > 9999 {
			return c, &Invalid{"actual_start_date", "Use a valid date in YYYY-MM-DD format"}
		}
	}
	if c.ActualEndDate != "" {
		if _, e := time.Parse("2006-01-02", c.ActualEndDate); e != nil {
			return c, &Invalid{"actual_end_date", "Use a valid actual end date"}
		}
		if c.ActualStartDate == "" || c.ActualEndDate < c.ActualStartDate {
			return c, &Invalid{"actual_end_date", "Actual end requires an actual start and cannot precede it"}
		}
	}
	if c.SiteHandoverDate != "" {
		v, e := time.Parse("2006-01-02", c.SiteHandoverDate)
		if e != nil || v.Year() < 1 || v.Year() > 9999 {
			return c, &Invalid{"site_handover_date", "Use a valid date in YYYY-MM-DD format"}
		}
	}
	basisDates := map[string]string{"manual": c.PlannedStartDate, "loi_date": c.LOIDate, "po_date": c.PODate, "site_handover_date": c.SiteHandoverDate, "actual_start_date": c.ActualStartDate}
	if c.ScheduleStartBasis == "" {
		c.ScheduleStartBasis = "loi_date"
	}
	selected, ok := basisDates[c.ScheduleStartBasis]
	if !ok {
		return c, &Invalid{"schedule_start_basis", "Select a start date basis"}
	}
	if !complete && (selected == "" || (c.PlannedDurationDays == "" && c.PlannedCompletionDate == "")) {
		return c, nil
	}
	if selected == "" {
		field := c.ScheduleStartBasis
		if field == "manual" {
			field = "planned_start_date"
		}
		return c, &Invalid{field, "The date matching the selected basis is required"}
	}
	if c.PlannedStartDate == "" {
		c.PlannedStartDate = selected
	}
	start, _ := time.Parse("2006-01-02", c.PlannedStartDate)
	if c.PlannedCompletionDate == "" {
		finish := start.AddDate(0, 0, duration-1)
		if finish.Year() > 9999 {
			return c, &Invalid{"planned_duration_days", "Calculated finish exceeds supported dates"}
		}
		c.PlannedCompletionDate = finish.Format("2006-01-02")
	}
	finish, _ := time.Parse("2006-01-02", c.PlannedCompletionDate)
	if finish.Before(start) || finish.Sub(start) > 36500*24*time.Hour {
		return c, &Invalid{"planned_completion_date", "Completion must follow start and be within 100 years"}
	}
	if c.SiteHandoverDate != "" && c.SiteHandoverDate < c.PlannedCompletionDate {
		return c, &Invalid{"site_handover_date", "Site handover must be on or after planned completion"}
	}
	return c, nil
}
