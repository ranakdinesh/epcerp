"use client";

import {
  ControlRow,
  ControlEdge,
  day,
  iso,
  overdue,
} from "./project-control-model";
import { processStatus } from "./project-process-data";

export function ProjectControlTimeline({
  rows,
  allRows,
  links,
  selected,
  impacted,
  today,
  expanded,
  onSelect,
  onToggle,
}: {
  rows: ControlRow[];
  allRows: ControlRow[];
  links: ControlEdge[];
  selected: string;
  impacted: Set<string>;
  today: string;
  expanded: Set<string>;
  onSelect: (id: string) => void;
  onToggle: (id: string) => void;
}) {
  const dates = allRows
    .flatMap((r) => [r.start, r.finish])
    .filter((d): d is string => !!d)
    .map(day);
  const start = Math.min(day(today), ...dates) - 3;
  const end = Math.max(day(today) + 14, ...dates) + 3;
  const span = end - start + 1;
  const x = (d: string) => ((day(d) - start) / span) * 100;
  const width = 600;
  const rowHeight = 76;
  const index = new Map(rows.map((r, i) => [r.id, i]));
  const visibleEdges = links.filter(
    (e) =>
      (e.from === selected || impacted.has(e.from)) &&
      index.has(e.from) &&
      index.has(e.to),
  );
  if (!rows.length)
    return (
      <p
        role="status"
        className="border-t border-slate-200 p-6 text-sm text-slate-600"
      >
        No work matches this filter. Choose All work to review the plan.
      </p>
    );
  return (
    <div
      role="region"
      aria-label="Draft project timeline; scroll horizontally to inspect dates"
      tabIndex={0}
      className="max-h-[680px] overflow-auto border-t border-slate-200 focus-visible:outline-2 focus-visible:outline-blue-600"
    >
      <div
        className="relative min-w-[980px]"
        role="table"
        aria-label="Processes and scheduled tasks"
      >
        <div
          role="row"
          className="sticky top-0 z-30 flex h-12 border-b border-slate-300 bg-slate-100"
        >
          <div
            role="columnheader"
            className="sm:sticky left-0 z-40 w-[380px] shrink-0 bg-slate-100 px-4 py-4 text-xs font-bold uppercase text-slate-600"
          >
            Process / task · owner · status
          </div>
          <div
            role="columnheader"
            className="relative min-w-[600px] flex-1 text-[11px] text-slate-600"
          >
            {Array.from({ length: 6 }, (_, i) => (
              <span
                key={i}
                className="absolute top-4"
                style={{ left: `${i * 17}%` }}
              >
                {iso(start + Math.floor(span * i * 0.17))}
              </span>
            ))}
          </div>
        </div>
        <div className="relative">
          {rows.map((row) => {
            const hasDates = !!row.start && !!row.finish;
            const children = allRows.filter((r) => r.parent === row.id);
            const active = selected === row.id;
            const exposed =
              impacted.has(row.id) &&
              !["completed", "bypassed"].includes(row.status);
            const color =
              row.status === "completed"
                ? "bg-emerald-600"
                : row.status === "blocked" || overdue(row, today) > 0
                  ? "bg-red-700"
                  : "bg-blue-600";
            return (
              <div
                key={row.id}
                role="row"
                className={`flex border-b border-slate-100 ${active ? "bg-blue-50" : exposed ? "bg-amber-50" : "bg-white"}`}
                style={{ height: rowHeight }}
              >
                <div
                  role="cell"
                  className={`sm:sticky left-0 z-20 flex w-[380px] shrink-0 items-center border-r border-slate-200 px-3 ${active ? "bg-blue-50" : exposed ? "bg-amber-50" : row.kind === "process" ? "bg-slate-50" : "bg-white"}`}
                >
                  {children.length > 0 ? (
                    <button
                      aria-label={`${expanded.has(row.id) ? "Collapse" : "Expand"} ${row.name}`}
                      aria-expanded={expanded.has(row.id)}
                      onClick={() => onToggle(row.id)}
                      className="mr-1 min-h-11 min-w-9 rounded-lg text-slate-700 hover:bg-slate-200"
                    >
                      {expanded.has(row.id) ? "▾" : "▸"}
                    </button>
                  ) : (
                    <span className="w-10 shrink-0 text-center text-slate-400">
                      ↳
                    </span>
                  )}
                  <button
                    onClick={() => onSelect(row.id)}
                    aria-pressed={active}
                    className="min-h-11 min-w-0 flex-1 text-left focus-visible:outline-2 focus-visible:outline-blue-600"
                  >
                    <span
                      className="block truncate text-sm font-bold text-slate-900"
                      title={row.name}
                    >
                      {row.name}
                    </span>
                    <span className="mt-1 block truncate text-xs text-slate-600">
                      {row.owner} · {processStatus(row.status)}
                      {row.kind === "process"
                        ? ` · ${children.length} tasks`
                        : ""}
                      {exposed ? " · Dependent" : ""}
                    </span>
                  </button>
                </div>
                <div
                  role="cell"
                  className="relative min-w-[600px] flex-1"
                  style={{
                    backgroundImage:
                      "linear-gradient(to right, #e2e8f0 1px, transparent 1px)",
                    backgroundSize: "17% 100%",
                  }}
                >
                  <span
                    className="pointer-events-none absolute inset-y-0 border-l-2 border-dashed border-slate-500"
                    style={{ left: `${x(today)}%` }}
                  >
                    {row === rows[0] ? (
                      <span className="absolute top-0 bg-white px-1 text-[10px] font-bold">
                        Today
                      </span>
                    ) : null}
                  </span>
                  {hasDates ? (
                    <button
                      onClick={() => onSelect(row.id)}
                      aria-label={`${row.name}: ${row.start} to ${row.finish}, ${processStatus(row.status)}`}
                      className={`absolute top-6 z-10 h-5 min-w-2 rounded ${color} ${active ? "ring-2 ring-blue-900 ring-offset-2" : ""} ${exposed ? "ring-2 ring-amber-500 ring-offset-1" : ""}`}
                      style={{
                        left: `${x(row.start!)}%`,
                        width: `${Math.max(0.5, ((day(row.finish!) - day(row.start!) + 1) / span) * 100)}%`,
                      }}
                    >
                      {row.milestone ? (
                        <span className="absolute -top-1 left-0 text-xl text-blue-950">
                          ◆
                        </span>
                      ) : null}
                    </button>
                  ) : (
                    <button
                      onClick={() => onSelect(row.id)}
                      className="absolute left-4 top-3 min-h-11 text-xs text-slate-500"
                    >
                      {row.start || row.finish
                        ? "Incomplete dates"
                        : "Not scheduled"}
                    </button>
                  )}
                  {hasDates ? (
                    <span className="absolute bottom-1 left-3 text-[10px] text-slate-600">
                      {row.start} → {row.finish}
                      {row.milestone ? " · Milestone" : ""}
                    </span>
                  ) : null}
                </div>
              </div>
            );
          })}
          <svg
            aria-hidden="true"
            className="pointer-events-none absolute bottom-0 right-0 top-0 z-10 h-full"
            style={{ width: "calc(100% - 380px)" }}
            viewBox={`0 0 ${width} ${rows.length * rowHeight}`}
            preserveAspectRatio="none"
          >
            <defs>
              <marker
                id="control-arrow"
                viewBox="0 0 10 10"
                refX="9"
                refY="5"
                markerWidth="5"
                markerHeight="5"
                orient="auto-start-reverse"
              >
                <path d="M 0 0 L 10 5 L 0 10 z" fill="#b45309" />
              </marker>
            </defs>
            {visibleEdges.map((e) => {
              const a = rows[index.get(e.from)!],
                b = rows[index.get(e.to)!];
              if (!a.finish || !a.start || !b.start || !b.finish) return null;
              const fromDate = e.relationship.startsWith("start")
                ? a.start
                : a.finish;
              const toDate = e.relationship.endsWith("finish")
                ? b.finish
                : b.start;
              const ax = (x(fromDate) / 100) * width,
                bx = (x(toDate) / 100) * width,
                ay = index.get(a.id)! * rowHeight + 34,
                by = index.get(b.id)! * rowHeight + 34;
              return (
                <path
                  key={`${e.from}-${e.to}`}
                  d={`M${ax},${ay} C${ax + 18},${ay} ${bx - 18},${by} ${bx},${by}`}
                  stroke="#b45309"
                  strokeWidth="1.5"
                  fill="none"
                  markerEnd="url(#control-arrow)"
                />
              );
            })}
          </svg>
        </div>
        {!rows.length ? (
          <p className="p-8 text-sm text-slate-600">
            No work matches this filter. Choose All work to review the plan.
          </p>
        ) : null}
      </div>
    </div>
  );
}
