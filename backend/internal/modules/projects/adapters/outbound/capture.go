package outbound

import (
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
)

func decimal(v pgtype.Numeric) string {
	if !v.Valid {
		return ""
	}
	value, _ := v.Value()
	return fmt.Sprint(value)
}
func date(v pgtype.Date) string {
	if !v.Valid {
		return ""
	}
	return v.Time.Format("2006-01-02")
}
func integer(v pgtype.Int4) string {
	if !v.Valid {
		return ""
	}
	return fmt.Sprint(v.Int32)
}
func capture(v db.Project) domain.Capture {
	var calc domain.Calculation
	_ = json.Unmarshal(v.ScheduleCalculation, &calc)
	return domain.Capture{PurchaseEnabled: v.PurchaseEnabled, ActualEndDate: date(v.ActualEndDate), Calculation: calc, ShortName: v.ShortName, ProjectType: v.ProjectType, SiteName: v.SiteName, SiteState: v.SiteState, SiteCity: v.SiteCity, ScopeSummary: v.ScopeSummary, OperationsManagerID: key(v.OperationsManagerID), ContractValue: decimal(v.ContractValue), Currency: v.Currency, LOINumber: v.LoiNumber, PONumber: v.PoNumber, ScheduleStartBasis: v.ScheduleStartBasis, PlannedStartDate: date(v.PlannedStartDate), PlannedCompletionDate: date(v.PlannedCompletionDate), LOIDate: date(v.LoiDate), PODate: date(v.PoDate), ActualStartDate: date(v.ActualStartDate), SiteHandoverDate: date(v.SiteHandoverDate), PlannedDurationDays: integer(v.PlannedDurationDays), SafetyBufferPercent: decimal(v.SafetyBufferPercent), SafetyBufferDays: integer(v.SafetyBufferDays), Remarks: v.Remarks}
}
