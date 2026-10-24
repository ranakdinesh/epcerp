package domain

import (
	"strconv"
	"strings"
	"time"
)

type WorkingCount struct{ CalendarDays, WeekendDays, HolidayDays, WorkingDays int }
type Calculation struct {
	ContractualStart    string `json:"contractual_start"`
	ContractualStartGap *int   `json:"contractual_start_gap"`
	PlannedStartGap     *int   `json:"planned_start_gap"`
	CalendarDays        *int   `json:"calendar_days"`
	WeekendDays         *int   `json:"weekend_days"`
	HolidayDays         *int   `json:"holiday_days"`
	CompletionDays      *int   `json:"completion_days"`
	ReservedBufferDays  *int   `json:"reserved_buffer_days"`
	ExecutionDays       *int   `json:"execution_days"`
}

func Gap(a, b string) *int {
	if a == "" || b == "" {
		return nil
	}
	x, _ := time.Parse("2006-01-02", a)
	y, _ := time.Parse("2006-01-02", b)
	n := int((x.Unix() - y.Unix()) / 86400)
	return &n
}
func (c Capture) ContractualStart() string {
	switch c.ScheduleStartBasis {
	case "loi_date":
		return c.LOIDate
	case "po_date":
		return c.PODate
	case "actual_start_date":
		return c.ActualStartDate
	case "site_handover_date":
		return c.SiteHandoverDate
	default:
		return c.PlannedStartDate
	}
}
func (c Capture) Calculate(w *WorkingCount) Calculation {
	r := Calculation{ContractualStart: c.ContractualStart(), ContractualStartGap: Gap(c.ActualStartDate, c.ContractualStart()), PlannedStartGap: Gap(c.ActualStartDate, c.PlannedStartDate)}
	if w == nil {
		return r
	}
	r.CalendarDays = &w.CalendarDays
	r.WeekendDays = &w.WeekendDays
	r.HolidayDays = &w.HolidayDays
	r.CompletionDays = &w.WorkingDays
	p := strings.Split(c.SafetyBufferPercent, ".")
	whole, _ := strconv.Atoi(p[0])
	fraction := 0
	if len(p) > 1 {
		fraction, _ = strconv.Atoi((p[1] + "00")[:2])
	}
	basisPoints := whole*100 + fraction
	reserved := (w.WorkingDays*basisPoints + 9999) / 10000
	remaining := w.WorkingDays - reserved
	r.ReservedBufferDays = &reserved
	r.ExecutionDays = &remaining
	return r
}

func (c Capture) WithWorkingCount(w *WorkingCount) Capture {
	c.Calculation = c.Calculate(w)
	if w == nil {
		c.PlannedDurationDays = ""
		c.SafetyBufferDays = ""
	} else {
		c.PlannedDurationDays = strconv.Itoa(w.WorkingDays)
		c.SafetyBufferDays = strconv.Itoa(*c.Calculation.ExecutionDays)
	}
	return c
}
