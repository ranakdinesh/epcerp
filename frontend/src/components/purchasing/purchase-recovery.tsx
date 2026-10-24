"use client";
import { ValidatedLabel, ValidatedTextarea } from "@/components/ui/validated-controls";

import { useEffect, useState } from "react";
import { useProjectUser } from "@/components/projects/project-permissions";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { request, button, input, secondary } from "./purchase-data";

type Source = { sheet: string; row: number; cells: Record<string, string> };
type Summary = {
  id: string;
  legacy_pid: string;
  name: string;
  filename: string;
  checksum: string;
  project_id: string | null;
  version: number;
  package_count: number;
  closed_count: number;
  issue_count: number;
};
type Detail = Omit<
  Summary,
  "package_count" | "closed_count" | "issue_count"
> & {
  review_reason: string;
  snapshot: {
    issues: string[];
    project_sources: Source[];
    packages: {
      uid: string;
      description: string;
      closed: boolean;
      indent_reference: string;
      source: Source;
    }[];
    events: {
      step: number;
      identity: string;
      kind: string;
      timestamp: string;
      source: Source;
    }[];
    steps: { number: number; phase: string; title: string }[];
    evidence: Source[];
  };
};

function SourceEvidence({ row }: { row: Source }) {
  return (
    <details className="rounded-lg border border-slate-200 bg-white p-3">
      <summary className="cursor-pointer font-medium">
        {row.sheet} · row {row.row}
      </summary>
      <dl className="mt-3 space-y-2 text-sm">
        {Object.entries(row.cells).map(([column, value]) => (
          <div key={column} className="flex gap-3">
            <dt className="w-8 shrink-0 font-bold">{column}</dt>
            <dd className="min-w-0 break-words [overflow-wrap:anywhere]">
              {value}
            </dd>
          </div>
        ))}
      </dl>
    </details>
  );
}

export function PurchaseRecovery({ project }: { project: string }) {
  const user = useProjectUser();
  const canWrite =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("admin.purchase_recovery.write");
  const [rows, setRows] = useState<Summary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const [selected, setSelected] = useState<string | null>(null);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [detailError, setDetailError] = useState("");
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    let live = true;
    request<Summary[]>("/api/purchase-recovery")
      .then((data) => {
        if (live) {
          setRows(data);
          setError("");
          setLoading(false);
        }
      })
      .catch((e) => {
        if (live) {
          setError(e.message);
          setLoading(false);
        }
      });
    return () => {
      live = false;
    };
  }, [reload]);
  useEffect(() => {
    if (!selected) return;
    let live = true;
    request<Detail>(`/api/purchase-recovery/${selected}`)
      .then((data) => {
        if (live) setDetail(data);
      })
      .catch((e) => {
        if (live) setDetailError(e.message);
      });
    return () => {
      live = false;
    };
  }, [selected]);
  function open(id: string) {
    setDetail(null);
    setDetailError("");
    setReason("");
    setNotice("");
    setSelected(id);
  }
  async function confirm() {
    if (!detail) return;
    setSaving(true);
    setDetailError("");
    setNotice("");
    try {
      const updated = await request<Detail>(
        `/api/purchase-recovery/${detail.id}/project-link`,
        { project_id: project, version: detail.version, reason },
      );
      setDetail(updated);
      setReload((n) => n + 1);
      setReason("");
      setNotice(
        "Project link saved. BOM and historical milestone review remain open.",
      );
    } catch (e) {
      setDetailError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }
  return (
    <section className="space-y-4">
      <div>
        <h3 className="text-lg font-bold">Historical purchase review</h3>
        <p className="mt-1 text-sm text-slate-600">
          Review source projects across the workbook and confirm which belongs
          to this project. Package descriptions still need BOM quantities, units
          and item mapping.
        </p>
      </div>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      <button
        className={secondary}
        onClick={() => {
          setLoading(true);
          setReload((n) => n + 1);
        }}
      >
        Refresh sources
      </button>
      <DataGrid
        label="Historical projects"
        rows={rows}
        loading={loading}
        status={(r) =>
          r.project_id === project
            ? "Linked to this project"
            : r.project_id
              ? "Linked elsewhere"
              : "Needs project review"
        }
        columns={[
          { key: "pid", label: "Source project", value: (r) => r.legacy_pid },
          { key: "name", label: "Project name", value: (r) => r.name },
          { key: "packages", label: "Packages", value: (r) => r.package_count },
          {
            key: "closed",
            label: "Marked closed",
            value: (r) => r.closed_count,
          },
          {
            key: "issues",
            label: "Review issues",
            value: (r) => r.issue_count,
          },
          {
            key: "link",
            label: "Project link",
            value: (r) =>
              r.project_id === project
                ? "This project"
                : r.project_id
                  ? "Another project"
                  : "Unconfirmed",
          },
        ]}
        actions={(r) => (
          <button
            className={secondary}
            aria-label={`Review ${r.legacy_pid}`}
            onClick={() => open(r.id)}
          >
            Review
          </button>
        )}
      />
      {selected && (
        <RecordDrawer
          title={
            detail
              ? `${detail.legacy_pid} · ${detail.name}`
              : "Loading historical project"
          }
          onClose={() => {
            if (
              !saving &&
              (!reason || window.confirm("Discard the unsaved review reason?"))
            )
              setSelected(null);
          }}
        >
          {detailError && (
            <p role="alert" className="text-red-700">
              {detailError}
            </p>
          )}
          {!detail && !detailError && (
            <p role="status">Loading source evidence…</p>
          )}
          {detail && (
            <>
              <p className="text-sm text-slate-600">
                {detail.filename}. Earlier project stages remain historically
                unverified. Source submissions do not establish approval or
                payment.
              </p>
              <div className="rounded-xl border border-amber-200 bg-amber-50 p-4">
                <h3 className="font-bold">Before recovering the BOM</h3>
                <ul className="mt-2 list-disc space-y-1 pl-5 text-sm">
                  {detail.snapshot.issues.map((issue, i) => (
                    <li key={i}>{issue}</li>
                  ))}
                </ul>
              </div>
              {detail.snapshot.project_sources.map((row, i) => (
                <SourceEvidence key={i} row={row} />
              ))}
              <DataGrid
                label="Source material packages"
                rows={detail.snapshot.packages.map((p) => ({
                  ...p,
                  id: `${p.uid}-${p.source.row}`,
                }))}
                columns={[
                  { key: "uid", label: "Package", value: (p) => p.uid },
                  {
                    key: "description",
                    label: "Material / kit",
                    value: (p) => p.description,
                  },
                  {
                    key: "quantity",
                    label: "Quantity / unit",
                    value: () => "Needs indent / BOQ evidence",
                  },
                  {
                    key: "status",
                    label: "Source status",
                    value: (p) =>
                      p.closed ? "Marked closed" : "No close flag",
                  },
                ]}
                actions={() => null}
              />
              <div className="space-y-3">
                <h3 className="font-bold">
                  Purchase history in source step order
                </h3>
                {detail.snapshot.steps.map((step) => {
                  const events = detail.snapshot.events.filter(
                    (e) => e.step === step.number,
                  );
                  return (
                    <details
                      key={step.number}
                      className="rounded-lg border border-slate-200 p-3"
                    >
                      <summary className="cursor-pointer text-sm font-medium">
                        {step.number}. {step.title} · {events.length} source
                        submissions
                      </summary>
                      {events.length ? (
                        events.map((event, i) => (
                          <div className="mt-3 space-y-2" key={i}>
                            <p className="text-sm">
                              {event.identity} ·{" "}
                              {event.kind.replaceAll("_", " ")} ·{" "}
                              {event.timestamp || "Timestamp unavailable"}
                            </p>
                            <SourceEvidence row={event.source} />
                          </div>
                        ))
                      ) : (
                        <p className="mt-2 text-sm text-slate-600">
                          No source submission recovered. Completion is
                          unverified.
                        </p>
                      )}
                    </details>
                  );
                })}
              </div>
              <details>
                <summary className="cursor-pointer font-bold">
                  All related source rows ({detail.snapshot.evidence.length})
                </summary>
                <div className="mt-3 space-y-2">
                  {detail.snapshot.evidence.map((row, i) => (
                    <SourceEvidence key={i} row={row} />
                  ))}
                </div>
              </details>
              <div className="space-y-3 border-t border-slate-200 pt-4">
                <h3 className="font-bold">Confirm project identity</h3>
                <p className="text-sm">
                  {detail.project_id === project
                    ? "Currently linked to this project."
                    : detail.project_id
                      ? "Currently linked to another project. Saving changes that link."
                      : "No project link has been confirmed."}
                </p>
                {detail.review_reason && (
                  <p className="text-sm">Last review: {detail.review_reason}</p>
                )}
                {canWrite && (
                  <>
                    <ValidatedLabel className="block text-sm font-medium">
                      Evidence that this source belongs to the current project
                      <ValidatedTextarea name="reason"
                        className={input}
                        value={reason}
                        maxLength={2000}
                        onChange={(e) => setReason(e.target.value)}
                      />
                    </ValidatedLabel>
                    <button
                      className={button}
                      disabled={saving || reason.trim().length < 10}
                      onClick={confirm}
                    >
                      {saving ? "Saving…" : "Confirm link to this project"}
                    </button>
                  </>
                )}
                {notice && (
                  <p role="status" className="text-sm text-emerald-800">
                    {notice}
                  </p>
                )}
              </div>
            </>
          )}
        </RecordDrawer>
      )}
    </section>
  );
}
