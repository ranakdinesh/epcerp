"use client";
import { useState, type ReactNode } from "react";
export const chartColors = [
  "#164fc7",
  "#168366",
  "#38749a",
  "#111c2d",
  "#8395a5",
];
export function ChartPanel({
  title,
  subtitle,
  source = "Recorded",
  children,
}: {
  title: string;
  subtitle?: string;
  source?: string;
  children: ReactNode;
}) {
  return (
    <section className="min-w-0 rounded-2xl border border-slate-200 border-t-[3px] border-t-[#164fc7] bg-white p-5 shadow-[0_10px_30px_-20px_rgba(17,28,45,0.3)]">
      <div className="mb-5 flex flex-wrap items-start justify-between gap-2">
        <div>
          <h3 className="font-bold text-slate-900">{title}</h3>
          {subtitle && (
            <p className="mt-1 text-xs leading-5 text-slate-500">{subtitle}</p>
          )}
        </div>
        <span
          className={`rounded-md px-2 py-1 text-[11px] font-bold ${source === "Demo" ? "bg-amber-100 text-amber-900" : "bg-slate-100 text-slate-600"}`}
        >
          {source}
        </span>
      </div>
      {children}
    </section>
  );
}
export function Metric({
  label,
  value,
  note,
  tone = "navy",
}: {
  label: string;
  value: ReactNode;
  note: string;
  tone?: "navy" | "sage" | "orange" | "ice";
}) {
  const themes = {
    navy: "bg-[#162531] text-white",
    sage: "bg-[#dcebe1] text-[#1e4934]",
    orange: "bg-[#ffede5] text-[#963511]",
    ice: "bg-[#d4e4f5] text-[#153c5a]",
  };
  return (
    <div
      data-tone={tone}
      className={`dashboard-metric relative min-w-0 overflow-hidden rounded-2xl p-5 shadow-sm ${themes[tone]}`}
    >
      <div aria-hidden="true" className="absolute right-5 top-5 flex gap-1">
        <span className="h-2 w-2 rounded-full bg-current opacity-25" />
        <span className="h-2 w-2 rounded-full bg-current opacity-50" />
        <span className="h-2 w-2 rounded-full bg-current" />
      </div>
      <p className="pr-10 text-xs font-semibold">{label}</p>
      <p className="my-3 break-words text-3xl font-bold tracking-tight">
        {value}
      </p>
      <p className="text-xs opacity-90">{note}</p>
    </div>
  );
}
export function CompositionChart({
  data,
  label,
  pie = false,
}: {
  data: { label: string; value: number; color?: string }[];
  label: string;
  pie?: boolean;
}) {
  const [selected, setSelected] = useState<number | null>(null);
  const total = data.reduce((sum, d) => sum + Math.max(0, d.value), 0);
  const radius = pie ? 46 : 70,
    circumference = 2 * Math.PI * radius;
  const selectedRow = selected === null ? null : data[selected];
  return (
    <div className="grid items-center gap-6 sm:grid-cols-[minmax(160px,1fr)_minmax(150px,1fr)]">
      <div className="relative mx-auto w-full max-w-[240px]">
        <svg
          viewBox="0 0 220 220"
          role="img"
          aria-label={`${label}: ${total} total. ${data.map((d) => `${d.label}: ${d.value}`).join(", ")}`}
          className="w-full"
        >
          <title>{label}</title>
          <circle
            cx="110"
            cy="110"
            r={radius}
            fill="none"
            stroke="#edf2f6"
            strokeWidth={pie ? 92 : 27}
          />
          {total > 0 &&
            data.map((d, i) => {
              const fraction = Math.max(0, d.value) / total;
              const preceding =
                data
                  .slice(0, i)
                  .reduce((sum, item) => sum + Math.max(0, item.value), 0) /
                total;
              return fraction > 0 ? (
                <circle
                  key={d.label}
                  cx="110"
                  cy="110"
                  r={radius}
                  fill="none"
                  stroke={d.color ?? chartColors[i % chartColors.length]}
                  strokeWidth={pie ? 92 : selected === i ? 33 : 27}
                  strokeDasharray={`${fraction * circumference} ${circumference}`}
                  strokeDashoffset={-preceding * circumference}
                  transform="rotate(-90 110 110)"
                  opacity={selected === null || selected === i ? 1 : 0.25}
                />
              ) : null;
            })}
          {!pie && (
            <>
              <text
                x="110"
                y="110"
                textAnchor="middle"
                fontSize="32"
                fontWeight="700"
                fill="#111c2d"
              >
                {selectedRow?.value ?? total}
              </text>
              <text
                x="110"
                y="134"
                textAnchor="middle"
                fontSize="12"
                fill="#526371"
              >
                {total === 0
                  ? "No records"
                  : selectedRow
                    ? "Selected"
                    : "Total"}
              </text>
            </>
          )}
        </svg>
      </div>
      <ul className="space-y-1">
        {data.map((d, i) => (
          <li key={d.label}>
            <button
              type="button"
              aria-pressed={selected === i}
              onClick={() => setSelected(selected === i ? null : i)}
              className={`flex min-h-11 w-full items-center gap-3 rounded-lg px-2 text-left text-sm focus-visible:outline-2 focus-visible:outline-[#164fc7] ${selected === i ? "bg-slate-100" : "hover:bg-slate-50"}`}
            >
              <span
                aria-hidden="true"
                className="h-3 w-3 shrink-0 rounded-full"
                style={{
                  background: d.color ?? chartColors[i % chartColors.length],
                }}
              />
              <span className="flex-1 capitalize text-slate-600">
                {d.label}
              </span>
              <strong className="text-slate-900">{d.value}</strong>
              <span className="w-12 text-right text-xs text-slate-500">
                {total ? Math.round((d.value / total) * 100) : 0}%
              </span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
export function Bars({
  data,
  unit = "",
  max,
}: {
  data: { label: string; value: number; color?: string }[];
  unit?: string;
  max?: number;
}) {
  const limit = max ?? Math.max(1, ...data.map((d) => d.value));
  return (
    <ul className="space-y-4">
      {data.map((d, i) => (
        <li key={d.label}>
          <div className="mb-1.5 flex justify-between gap-3 text-sm">
            <span className="text-slate-600">{d.label}</span>
            <strong className="whitespace-nowrap text-slate-900">
              {d.value.toLocaleString("en-IN")}
              {unit}
            </strong>
          </div>
          <div className="h-2.5 overflow-hidden rounded-full bg-slate-100">
            <div
              className="h-full rounded-full"
              style={{
                width: `${Math.min(100, Math.max(0, (d.value / limit) * 100))}%`,
                background: d.color ?? chartColors[i % chartColors.length],
              }}
            />
          </div>
        </li>
      ))}
    </ul>
  );
}
