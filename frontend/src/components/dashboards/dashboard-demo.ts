import type { ControlRow } from "@/components/projects/project-control-model";
// Presentation-only records. Never submit these to project APIs.
export const demoSchedule: ControlRow[] = [
  ["Survey & scope", "2026-08-03", "2026-08-14", "completed", "Engineering"],
  ["Design & drawings", "2026-08-10", "2026-09-04", "completed", "Design team"],
  [
    "Material procurement",
    "2026-08-24",
    "2026-09-25",
    "blocked",
    "Procurement",
  ],
  ["Site installation", "2026-09-07", "2026-10-16", "in_progress", "Site team"],
  [
    "Testing & inspection",
    "2026-10-12",
    "2026-10-30",
    "not_started",
    "Quality team",
  ],
  [
    "Commissioning & handover",
    "2026-10-26",
    "2026-11-06",
    "not_started",
    "Project manager",
  ],
].map(([name, start, finish, status, owner], i) => ({
  id: `example-${i}`,
  parent: null,
  kind: "process",
  name,
  owner,
  start,
  finish,
  status,
  milestone: false,
  actualStart: null,
  actualFinish: null,
  confirmed: false,
}));
