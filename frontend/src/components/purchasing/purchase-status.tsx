"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { request } from "./purchase-data";
type Status = {
  active_holds: { id: string; reason: string; started_at: string }[];
  lines: {
    id: string;
    name: string;
    policy: { status: string; reasons: string[] };
    holds: { id: string; reason: string }[];
    hold_minutes: number;
  }[];
};
export function PurchaseStatus({ project }: { project: string }) {
  const [status, setStatus] = useState<Status | null>(null),
    [error, setError] = useState("");
  useEffect(() => {
    let live = true;
    request<Status>(`/api/projects/${project}/purchase-status`)
      .then((s) => {
        if (live) setStatus(s);
      })
      .catch(() => {
        if (live) setError("Purchase hold / client policy status unavailable.");
      });
    return () => {
      live = false;
    };
  }, [project]);
  if (error)
    return (
      <p
        role="status"
        className="rounded-lg bg-amber-50 p-3 text-xs text-amber-900"
      >
        {error}
      </p>
    );
  if (!status)
    return (
      <p className="text-xs text-slate-500">
        Checking client purchase controls…
      </p>
    );
  const blocked = status.lines.filter((l) => l.policy.status !== "eligible");
  return (
    <aside
      className={`rounded-xl border p-4 ${status.active_holds.length ? "border-orange-300 bg-orange-50" : "border-slate-200 bg-white"}`}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm font-bold">
          Client purchase controls · {status.active_holds.length} active holds ·{" "}
          {blocked.length} items need make-policy review
        </p>
        <Link
          className="min-h-11 py-3 text-sm font-bold text-[#b73d13] underline"
          href={`/dashboard?screen=projects&project=${project}&tab=purchasing`}
        >
          View purchase controls
        </Link>
      </div>
      {status.active_holds.length > 0 && (
        <>
          <ul className="space-y-1 text-sm">
            {status.active_holds.map((h) => (
              <li key={h.id}>
                {h.reason} · since {new Date(h.started_at).toLocaleString()}
              </li>
            ))}
          </ul>
          <p className="mt-2 text-xs text-orange-900">
            Affected purchasing is blocked. Recorded hold time is shown per
            item; project-finish impact is unassessed and contractual dates are
            unchanged.
          </p>
        </>
      )}
    </aside>
  );
}
