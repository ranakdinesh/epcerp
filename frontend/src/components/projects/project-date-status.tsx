"use client";
import { useEffect, useState } from "react";
import { calendarDaysToTarget, projectDateVariance, type ProjectDates } from "./project-date-model";
export function useProjectToday() {
  const [today, setToday] = useState("");
  useEffect(() => {
    const update = () => setToday(new Intl.DateTimeFormat("en-CA", {timeZone: "Asia/Kolkata", year: "numeric", month: "2-digit", day: "2-digit"}).format(new Date()));
    update();
    const timer = window.setInterval(update, 60_000);
    window.addEventListener("focus", update);
    document.addEventListener("visibilitychange", update);
    return () => {window.clearInterval(timer); window.removeEventListener("focus", update); document.removeEventListener("visibilitychange", update);};
  }, []);
  return today;
}
export function ProjectDateStatus({project}: {project: ProjectDates}) {
  const today = useProjectToday();
  const status = projectDateVariance(project, today);
  const days = calendarDaysToTarget(project.actual_start_date || project.planned_start_date, project.planned_completion_date);
  return <section aria-label="Project schedule status" className={`rounded-xl border p-4 text-sm ${status.late ? "border-amber-300 bg-amber-50 text-amber-950" : "border-slate-200 bg-slate-50 text-slate-700"}`}>
    <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="font-bold">Schedule status</h3><span className="text-xs">As of {today || "…"} · Asia/Kolkata</span></div>
    {status.reasons.map(reason => <p key={reason} className="mt-2 font-semibold">{reason}</p>)}
    {!status.late && <p className="mt-2">{project.lifecycle_status === "closed" || project.is_active === false ? "Archived project — current delay tracking stopped." : !project.planned_start_date || !project.planned_completion_date ? "Set planned dates to assess schedule lateness." : "No date-based delay recorded."}</p>}
    <p className="mt-2">{days === null ? "Set start and planned completion to calculate running days." : `${days} calendar days from ${project.actual_start_date ? "actual" : "planned"} start to the fixed completion target (including the start day).`}</p>
    <p className="mt-2 text-xs">Start variance and completion overdue are separate date checks; they do not measure task progress.</p>
  </section>;
}
