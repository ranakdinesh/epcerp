export type ProjectDates = {
  planned_start_date: string | null;
  planned_completion_date: string | null;
  actual_start_date?: string | null;
  lifecycle_status?: string;
  is_active?: boolean;
};
const DAY = 86_400_000;
const daysBetween = (start: string, end: string) =>
  Math.round((Date.parse(`${end}T00:00:00Z`) - Date.parse(`${start}T00:00:00Z`)) / DAY);
export function calendarDaysToTarget(start: string | null, end: string | null) {
  return start && end ? Math.max(0, daysBetween(start, end) + 1) : null;
}
export function projectDateVariance(project: ProjectDates, today: string) {
  const current = project.is_active !== false && project.lifecycle_status !== "closed";
  const actual = project.actual_start_date && project.actual_start_date <= today
    ? project.actual_start_date : null;
  const startDays = current && project.planned_start_date && today
    ? Math.max(0, daysBetween(project.planned_start_date, actual ?? today)) : 0;
  const completionDays = current && project.planned_completion_date && today
    ? Math.max(0, daysBetween(project.planned_completion_date, today)) : 0;
  const reasons = [
    ...(completionDays ? [`Completion overdue by ${completionDays} calendar days`] : []),
    ...(startDays ? [`${actual ? "Started" : "Start overdue by"} ${startDays} calendar days${actual ? " late" : ""}`] : []),
  ];
  return {startDays, completionDays, reasons, late: reasons.length > 0};
}

type EditableDates = ProjectDates & {
  schedule_start_basis: string;
  loi_date: string | null;
  po_date: string | null;
  site_handover_date: string | null;
  actual_start_date: string | null;
  planned_duration_days: number | null;
  net_running_days: number | null;
};
export function recalculateProjectDates<T extends EditableDates>(current: T, key: keyof T, value: T[keyof T]): T {
  const next = {...current, [key]: value};
  const basis = next.schedule_start_basis as "loi_date" | "po_date" | "site_handover_date" | "actual_start_date";
  const basisChanged = key === "schedule_start_basis";
  const milestoneChanged = key === basis;
  const followsMilestone = !current.planned_start_date || current.planned_start_date === current[basis];
  if (next.schedule_start_basis !== "manual") {
    // Actual start measures execution against an existing plan; it cannot move that plan.
    if (basis === "actual_start_date") {
      if (key !== "planned_start_date") next.planned_start_date = next.planned_start_date || next.actual_start_date;
    } else if (basisChanged || (milestoneChanged && followsMilestone) || (!next.planned_start_date && key !== "planned_start_date")) {
      next.planned_start_date = next[basis];
    }
  }
  const actualChange = key === "actual_start_date" || (basisChanged && basis === "actual_start_date");
  const planChanged = key === "planned_duration_days" || key === "planned_start_date" || next.planned_start_date !== current.planned_start_date;
  // Repair a missing target from the planned baseline, never from a later actual start.
  if (key !== "planned_completion_date" && (!next.planned_completion_date || (!actualChange && planChanged)) && next.planned_start_date && next.planned_duration_days) {
    const end = new Date(`${next.planned_start_date}T00:00:00Z`);
    end.setUTCDate(end.getUTCDate() + next.planned_duration_days - 1);
    next.planned_completion_date = end.toISOString().slice(0, 10);
  }
  if ((key === "planned_completion_date" || !next.planned_duration_days) && next.planned_start_date && next.planned_completion_date) {
    next.planned_duration_days = calendarDaysToTarget(next.planned_start_date, next.planned_completion_date) || null;
  }
  // A missing imported baseline must not prevent actual-start recalculation.
  // Keep the previous result visible until a separately keyed server response succeeds.
  return next;
}

export function handoverDateError(project: ProjectDates & {site_handover_date:string|null}) {
  return project.site_handover_date && project.planned_completion_date && project.site_handover_date < project.planned_completion_date
    ? "Site handover date must be on or after Planned completion." : "";
}
