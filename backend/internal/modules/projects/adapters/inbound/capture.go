package inbound

import (
	"encoding/json"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
	"strings"
)

type CaptureRequest struct {
	PurchaseEnabled       strictBoolean `json:"purchase_enabled"`
	ActualEndDate         string        `json:"actual_end_date"`
	ShortName             string        `json:"short_name"`
	ProjectType           string        `json:"project_type"`
	SiteName              string        `json:"site_name"`
	SiteState             string        `json:"site_state"`
	SiteCity              string        `json:"site_city"`
	ScopeSummary          string        `json:"scope_summary"`
	ContractValue         string        `json:"contract_value"`
	Currency              string        `json:"currency"`
	LOINumber             string        `json:"loi_number"`
	PONumber              string        `json:"po_number"`
	ScheduleStartBasis    string        `json:"schedule_start_basis"`
	PlannedStartDate      string        `json:"planned_start_date"`
	PlannedCompletionDate string        `json:"planned_completion_date"`
	LOIDate               string        `json:"loi_date"`
	PODate                string        `json:"po_date"`
	ActualStartDate       string        `json:"actual_start_date"`
	SiteHandoverDate      string        `json:"site_handover_date"`
	PlannedDurationDays   string        `json:"planned_duration_days"`
	SafetyBufferPercent   string        `json:"safety_buffer_percent"`
	SafetyBufferDays      string        `json:"safety_buffer_days"`
	Remarks               string        `json:"remarks"`
}

func (in CaptureRequest) capture() domain.Capture {
	return domain.Capture{PurchaseEnabled: bool(in.PurchaseEnabled), ActualEndDate: in.ActualEndDate, ShortName: in.ShortName, ProjectType: in.ProjectType, SiteName: in.SiteName, SiteState: in.SiteState, SiteCity: in.SiteCity, ScopeSummary: in.ScopeSummary, ContractValue: in.ContractValue, Currency: in.Currency, LOINumber: in.LOINumber, PONumber: in.PONumber, ScheduleStartBasis: in.ScheduleStartBasis, PlannedStartDate: in.PlannedStartDate, PlannedCompletionDate: in.PlannedCompletionDate, LOIDate: in.LOIDate, PODate: in.PODate, ActualStartDate: in.ActualStartDate, SiteHandoverDate: in.SiteHandoverDate, PlannedDurationDays: in.PlannedDurationDays, SafetyBufferPercent: in.SafetyBufferPercent, SafetyBufferDays: in.SafetyBufferDays, Remarks: in.Remarks}
}
func captureView(c domain.Capture) map[string]any {
	return map[string]any{"purchase_enabled": c.PurchaseEnabled, "short_name": c.ShortName, "project_type": c.ProjectType, "site_name": c.SiteName, "site_state": c.SiteState, "site_city": c.SiteCity, "scope_summary": c.ScopeSummary, "operations_manager_id": c.OperationsManagerID, "contract_value": c.ContractValue, "currency": c.Currency, "loi_number": c.LOINumber, "po_number": c.PONumber, "schedule_start_basis": c.ScheduleStartBasis, "planned_start_date": c.PlannedStartDate, "planned_completion_date": c.PlannedCompletionDate, "loi_date": c.LOIDate, "po_date": c.PODate, "actual_start_date": c.ActualStartDate, "site_handover_date": c.SiteHandoverDate, "planned_duration_days": c.PlannedDurationDays, "safety_buffer_percent": c.SafetyBufferPercent, "safety_buffer_days": c.SafetyBufferDays, "remarks": c.Remarks, "net_running_days": c.Calculation.CompletionDays, "actual_end_date": c.ActualEndDate, "schedule_calculation": c.Calculation}
}

// JSON null is not a valid project purchase enablement decision.
type strictBoolean bool

func (v *strictBoolean) UnmarshalJSON(raw []byte) error {
	if strings.TrimSpace(string(raw)) == "null" {
		return errors.New("expected a boolean")
	}
	var b bool
	if e := json.Unmarshal(raw, &b); e != nil {
		return e
	}
	*v = strictBoolean(b)
	return nil
}
