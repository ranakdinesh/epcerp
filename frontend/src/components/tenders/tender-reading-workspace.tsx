"use client";
import { createRecordId } from "@/lib/record-id";
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import {
  EyeIcon,
  PlusIcon,
  ArrowPathIcon,
  ArrowLeftIcon,
  ArrowUpRightIcon,
  ArrowRightIcon,
  CheckCircleIcon,
  DocumentCheckIcon,
  PencilSquareIcon,
  TrashIcon,
} from "@heroicons/react/24/outline";
import { processMapHref, tenderProcess } from "../processes/process-directory";
import { TenderPerformanceBand } from "../dashboard/tender-performance-band";
import { useDashboardSource } from "../dashboard/use-dashboard-source";
import type { TenderSummary } from "@/server/tenders/summary";
import "../dashboard/procurement-dashboard.css";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { DataGrid, type GridQuery } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import {
  ValidatedForm,
  ValidatedInput,
  ValidatedLabel,
} from "@/components/ui/validated-controls";
import { CloudTSCWorkspace } from "./cloud-tsc-workspace";
import { TenderDocumentIntake } from "./tender-document-intake";
import type { TenderDocument } from "./tender-document-list";
import { TenderBiddingApproval } from "./tender-bidding-approval";
import { TenderPrebidQueries } from "./tender-prebid-queries";
import { TenderBOQItems } from "./tender-boq-items";
import { TenderVendorComparison } from "./tender-vendor-comparison";
import { TenderBidCosting } from "./tender-bid-costing";
import { TenderAwardHandoff } from "./tender-award-handoff";

type Stage =
  | "details"
  | "replies"
  | "documents"
  | "tsc"
  | "bidding"
  | "queries"
  | "boq"
  | "quotes"
  | "costing"
  | "award";

function stageForStep(step?: string): Stage {
  if (step === "LG-01") return "details";
  if (step === "LG-04") return "replies";
  if (step === "TS-02" || step === "LG-02") return "bidding";
  if (step === "TS-TSC") return "tsc";
  if (step === "LG-03") return "queries";
  if (step === "CV-01" || step === "CV-02") return "boq";
  if (step === "CV-03" || step === "CV-04" || step === "CV-05") return "quotes";
  if (step === "CV-07") return "costing";
  if (step === "CV-08" || step === "AWARD") return "award";
  return "documents";
}

function stepForStage(stage: Stage): string {
  switch (stage) {
    case "details": return "LG-01";
    case "replies": return "LG-04";
    case "documents": return "TS-01";
    case "tsc": return "TS-TSC";
    case "bidding": return "TS-02";
    case "queries": return "LG-03";
    case "boq": return "CV-01";
    case "quotes": return "CV-03";
    case "costing": return "CV-07";
    case "award": return "CV-08";
  }
}

type Source = {
  id: string;
  member_id: string;
  path: string;
  filename: string;
  sha256: string;
  status: string;
  error: string;
  version: number;
  attempts: number;
  segments: number;
  expected_pages: number;
  expected_sheets: number;
  warnings: string[];
  updated_at: string;
};
type Batch = {
  documents?: TenderDocument[];
  manifest: { path: string; state: string; code: string }[];
  id: string;
  revision: string;
  state: string;
  error: string;
  version: number;
  attempts: number;
  files: {
    id: string;
    name: string;
    kind?: string;
    archive_deleted_at?: string | null;
    state: string;
    error: string;
    sha256: string;
    version: number;
  }[];
  sources: Source[];
};
const basicFields = [
  { key: "reference", label: "Tender reference", type: "text", maxLength: 255 },
  { key: "authority", label: "Issuing authority", type: "text", maxLength: 255 },
  { key: "portal_url", label: "Tender portal URL", type: "url", maxLength: 2048 },
  { key: "due_date", label: "Bid submission date", type: "date", maxLength: 10 },
] as const;
type BasicDetails = Partial<Record<(typeof basicFields)[number]["key"], string>>;

type Draft = {
  engine?: string;
  id: string;
  name: string;
  version: number;
  created_at: string;
  updated_at: string;
  batches: Batch[];
  next_step?: string;
  status_label?: string;
  completed_steps?: number[];
  basic_details?: BasicDetails;
  has_verified_batches?: boolean;
  prebid_review_closed?: boolean;
  tsc_completed?: boolean;
  bidding_authorized?: boolean;
  created_by_id?: string;
  owner_id?: string;
  can_delete?: boolean;
  can_edit?: boolean;
};

function stepName(code: string): string {
  const entry = tenderProcess.steps.find(step => step.code === code);
  return entry ? `${entry.number}. ${entry.label}` : code;
}

const primary =
  "inline-flex min-h-10 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-xs font-bold text-white shadow-sm transition hover:bg-blue-800 disabled:opacity-50";
const secondary =
  "inline-flex min-h-10 items-center justify-center gap-1.5 rounded-xl border border-slate-300 bg-white px-3 text-xs font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700";
const icon =
  "inline-flex min-h-9 min-w-9 items-center justify-center rounded-xl text-blue-700 transition hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-slate-800";

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(`/api/tender-fms/${path}`, { cache: "no-store", ...init });
  if (r.status === 204) return undefined as T;
  const b = await r.json().catch(() => {
    if (init?.signal?.aborted) throw init.signal.reason;
    return { detail: "Unable to complete request" };
  });
  if (!r.ok) throw new Error(b.detail || b.message || `Request failed (${r.status})`);
  return b as T;
}

const json = (body: unknown, key?: string): RequestInit => ({
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    ...(key ? { "Idempotency-Key": key } : {}),
  },
  body: JSON.stringify(body),
});

const failure = (e: unknown) =>
  e instanceof Error ? e.message : "Unable to complete request";

export function TenderReadingWorkspace({
  assigned: _assigned,
  step,
  draftID,
  onSelectDraft,
  onSelectStep,
}: {
  assigned?: ReactNode;
  step?: string;
  draftID?: string;
  onSelectDraft?: (id: string | null, targetStep?: string) => void;
  onSelectStep?: (step: string) => void;
}) {
  const user = useAuthenticatedUser();
  const has = (p: string) =>
    user.permissions.includes("system:admin") || user.permissions.includes(p);
  const canRead =
    has("tenders.register.read") && has("tenders.processing.read");
  const canWrite =
    has("tenders.register.write") && has("tenders.processing.write");

  const [rows, setRows] = useState<Draft[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [add, setAdd] = useState(false);
  const [editingTender, setEditingTender] = useState<Draft | null>(null);
  const [deletingTender, setDeletingTender] = useState<Draft | null>(null);
  const [selected, setSelected] = useState<Draft | null>(null);
  const [summaryRevision, setSummaryRevision] = useState(0);
  const onDashboard = !draftID && !selected;
  const summary = useDashboardSource<TenderSummary>(
    canRead && onDashboard ? "/api/tender-fms/summary" : null, summaryRevision
  );
  const [page, setPage] = useState({
    page: 1,
    size: 20,
    search: "",
    sort: "created",
    direction: "desc",
  });

  const refresh = useCallback(
    async (signal?: AbortSignal) => {
      try {
        const v = await api<{ items: Draft[]; total: number }>(
          `drafts?page=${page.page}&page_size=${page.size}&search=${encodeURIComponent(
            page.search
          )}&sort=${page.sort}&direction=${page.direction}`,
          { signal }
        );
        setRows(v.items);
        setTotal(v.total);
        setError("");
      } catch (e) {
        if (!signal?.aborted) setError(failure(e));
      } finally {
        if (!signal?.aborted) setLoading(false);
      }
    },
    [page]
  );

  useEffect(() => {
    if (!canRead || !onDashboard) return;
    const c = new AbortController();
    const initial = setTimeout(() => void refresh(c.signal), 0);
    return () => {
      clearTimeout(initial);
      c.abort();
    };
  }, [canRead, onDashboard, refresh]);

  const onPage = useCallback(
    (q: GridQuery) =>
      setPage((p) =>
        p.page === q.page &&
        p.size === q.size &&
        p.search === q.search &&
        p.sort === (q.sort || "created") &&
        p.direction === q.direction
          ? p
          : {
              page: q.page,
              size: q.size,
              search: q.search,
              sort: q.sort || "created",
              direction: q.direction,
            }
      ),
    []
  );

  const detailDirty = useRef(false);
  const markDetailDirty = useCallback((value: boolean) => {
    detailDirty.current = value;
  }, []);

  const mayLeaveDetail = useCallback(() => {
    if (detailDirty.current && !window.confirm("Discard unsaved checklist changes?")) {
      return false;
    }
    markDetailDirty(false);
    return true;
  }, [markDetailDirty]);

  function refreshDashboard() {
    setLoading(true);
    setSummaryRevision(value => value + 1);
    void refresh();
  }

  function closeDetail() {
    if (!mayLeaveDetail()) return;
    setSelected(null);
    onSelectDraft?.(null);
    setLoading(true);
  }

  const stage = stageForStep(step);

  useEffect(() => {
    if (!canRead || !draftID) {
      if (!draftID) {
        const clear = setTimeout(() => setSelected(null), 0);
        return () => clearTimeout(clear);
      }
      return;
    }
    const controller = new AbortController();
    api<Draft>(`drafts/${encodeURIComponent(draftID)}`, { signal: controller.signal })
      .then((d) => {
        if (!controller.signal.aborted) {
          setSelected(d);
          if (!step && d.next_step && d.next_step !== "TS-01") {
            onSelectStep?.(d.next_step);
          }
        }
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(failure(e));
      });
    return () => controller.abort();
  }, [canRead, draftID, step, onSelectStep]);

  function openDraft(d: Draft, stepOverride?: string) {
    const target = stepOverride || d.next_step || "TS-01";
    setSelected(d);
    onSelectDraft?.(d.id, target);
  }

  if (!canRead) {
    return <p role="alert">Tender register and processing read permissions are required.</p>;
  }

  // --- Screen 2: Low-click Workspace View inside Tender FMS Process ---
  if (draftID || selected) {
    if (!selected) {
      return (
        <div className="flex min-h-64 flex-col items-center justify-center gap-3 rounded-2xl border border-slate-200 bg-white p-8 text-center dark:border-slate-800 dark:bg-slate-900">
          <ArrowPathIcon className="h-6 w-6 animate-spin text-blue-600" />
          <p className="text-sm font-semibold text-slate-700 dark:text-slate-300">
            Loading tender workspace…
          </p>
        </div>
      );
    }

    return (
      <DraftDetail
        key={`${selected.id}:${stage}`}
        draft={selected}
        canWrite={canWrite}
        initialStage={stage}
        activeStep={step}
        onSelectStep={onSelectStep}
        onDone={closeDetail}
        onDirty={markDetailDirty}
        onBeforeLeave={mayLeaveDetail}
      />
    );
  }

  return (
    <div className="go-procurement-dashboard">
      <header className="go-dashboard-heading">
        <div>
          <h1>Tender FMS</h1>
          <p>Track tender performance and continue work from the register.</p>
        </div>
        <div className="go-dashboard-heading-actions">
          <button type="button" className="ui-action ui-action--secondary" onClick={refreshDashboard}
            disabled={loading || summary.loading}>
            <ArrowPathIcon className="h-4 w-4" aria-hidden="true" />
            Refresh
          </button>
          {canWrite && (
            <button type="button" className="ui-action ui-action--primary" onClick={() => setAdd(true)}>
              <PlusIcon className="h-4 w-4" aria-hidden="true" />
              Add Tender
            </button>
          )}
        </div>
      </header>

      <TenderPerformanceBand data={summary.data} error={summary.error} />

      <section className="go-dashboard-panel go-dashboard-projects" aria-labelledby="tender-register-heading">
        <div className="go-dashboard-section-heading">
          <div>
            <h2 id="tender-register-heading">Tender register</h2>
            <p>Open a tender to continue at its current stage.</p>
          </div>
          <Link className="go-dashboard-text-link" href={processMapHref(tenderProcess)}>
            Reference process map <ArrowUpRightIcon aria-hidden="true" />
          </Link>
        </div>
        {error && (
          <div className="flex items-center justify-between rounded-xl border border-red-200 bg-red-50 p-4 text-xs font-semibold text-red-800 dark:border-red-900 dark:bg-red-950/60 dark:text-red-300">
            <span>{error}</span>
            <button
              type="button"
              className={secondary}
              onClick={() => void refresh()}
            >
              Retry
            </button>
          </div>
        )}

        <DataGrid
          stateKey="tender_drafts"
          label="Tenders"
          rows={rows}
          loading={loading}
          searchable
          remote={{ total, onChange: onPage }}
          onRowClick={(d) => openDraft(d, d.next_step)}
          columns={[
            {
              key: "name",
              label: "Tender Name",
              value: (d) => d.name,
              sortable: true,
              render: (d) => (
                <div>
                  <Link
                    href={`/dashboard?screen=tender-fms&draft=${encodeURIComponent(d.id)}&step=${encodeURIComponent(d.next_step || "TS-01")}`}
                    className="font-bold text-blue-700 hover:underline dark:text-blue-400"
                    onClick={(e) => {
                      e.stopPropagation();
                      openDraft(d, d.next_step);
                    }}
                  >
                    {d.name}
                  </Link>
                  {(d.basic_details?.reference || d.basic_details?.authority) && (
                    <p className="text-[11px] text-slate-500 dark:text-slate-400">
                      {[d.basic_details.reference, d.basic_details.authority].filter(Boolean).join(" · ")}
                    </p>
                  )}
                </div>
              ),
            },
            {
              key: "stage",
              label: "Current Stage",
              value: (d) => d.status_label || "Step 2 · Read & verify documents",
              sortable: false,
              render: (d) => (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-blue-50 px-2.5 py-0.5 text-xs font-semibold text-blue-800 dark:bg-blue-950/60 dark:text-blue-300">
                  <span className="h-1.5 w-1.5 rounded-full bg-blue-600" />
                  {d.status_label || "Step 2 · Read & verify documents"}
                </span>
              ),
            },
            {
              key: "next",
              label: "Active Action",
              value: (d) => d.next_step || "TS-01",
              sortable: false,
              render: (d) => (
                <span className="text-xs font-medium text-slate-700 dark:text-slate-300">
                  {stepName(d.next_step || "TS-01")}
                </span>
              ),
            },
            {
              key: "created",
              label: "Created Date",
              value: (d) => d.created_at,
              sortable: true,
              render: (d) => (
                <span className="text-xs text-slate-600 dark:text-slate-400">
                  {new Date(d.created_at).toLocaleDateString("en-IN", {
                    day: "2-digit",
                    month: "short",
                    year: "numeric",
                  })}
                </span>
              ),
            },
          ]}
          actions={(d) => {
            const canManage =
              user.permissions.includes("system:admin") ||
              (d.created_by_id && user.id === d.created_by_id) ||
              (d.owner_id && user.id === d.owner_id) ||
              Boolean(d.can_delete) ||
              Boolean(d.can_edit);
            return (
              <div className="flex items-center gap-1">
                <Link
                  href={`/dashboard?screen=tender-fms&draft=${encodeURIComponent(d.id)}&step=${encodeURIComponent(d.next_step || "TS-01")}`}
                  className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-blue-700 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-950"
                  title={`Continue ${d.name} at ${stepName(d.next_step || "TS-01")}`}
                  aria-label={`Continue ${d.name}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    openDraft(d, d.next_step);
                  }}
                >
                  <ArrowRightIcon className="h-5 w-5" />
                </Link>
                {canManage && (
                  <button
                    type="button"
                    className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800"
                    title={`Edit ${d.name}`}
                    aria-label={`Edit ${d.name}`}
                    onClick={(e) => {
                      e.stopPropagation();
                      setEditingTender(d);
                    }}
                  >
                    <PencilSquareIcon className="h-5 w-5" />
                  </button>
                )}
                {canManage && (
                  <button
                    type="button"
                    className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/60"
                    title={`Delete ${d.name}`}
                    aria-label={`Delete ${d.name}`}
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeletingTender(d);
                    }}
                  >
                    <TrashIcon className="h-5 w-5" />
                  </button>
                )}
              </div>
            );
          }}
        />

      </section>

      {add && (
        <RecordDrawer
          title="Add Tender"
          onClose={() => setAdd(false)}
        >
          <CreateDraft
            saved={(d) => {
              setAdd(false);
              openDraft(d, "LG-01");
            }}
          />
        </RecordDrawer>
      )}

      {editingTender && (
        <RecordDrawer
          title="Edit Tender"
          onClose={() => setEditingTender(null)}
        >
          <EditDraft
            draft={editingTender}
            saved={() => {
              setEditingTender(null);
              refreshDashboard();
            }}
            onCancel={() => setEditingTender(null)}
          />
        </RecordDrawer>
      )}

      {deletingTender && (
        <RecordDrawer
          title="Delete Tender"
          onClose={() => setDeletingTender(null)}
        >
          <DeleteDraft
            draft={deletingTender}
            deleted={() => {
              setDeletingTender(null);
              refreshDashboard();
            }}
            onCancel={() => setDeletingTender(null)}
          />
        </RecordDrawer>
      )}
    </div>
  );
}

function CreateDraft({ saved }: { saved: (d: Draft) => void }) {
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [locked, setLocked] = useState(false);
  const operation = useRef<string | null>(null);
  const submitted = useRef<string | null>(null);

  return (
    <ValidatedForm
      className="space-y-4"
      onSubmit={async (e) => {
        e.preventDefault();
        if (busy) return;
        setBusy(true);
        setError("");
        try {
          operation.current ??= createRecordId();
          submitted.current ??= name.trim();
          setLocked(true);
          saved(
            await api<Draft>("drafts", {
              ...json({ name: submitted.current }, operation.current),
              signal: AbortSignal.timeout(30000),
            })
          );
        } catch (e) {
          setError(
            e instanceof Error && e.name === "TimeoutError"
              ? "The server did not confirm creation within 30 seconds. Keep this panel open and retry to avoid duplicates."
              : failure(e)
          );
        } finally {
          setBusy(false);
        }
      }}
    >
      <ValidatedLabel required htmlFor="draft-name" className="text-xs font-bold text-slate-700 dark:text-slate-300">
        Tender Name
        <ValidatedInput
          id="draft-name"
          name="name"
          required
          minLength={2}
          maxLength={255}
          value={name}
          disabled={busy || locked}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. 50 MW Solar Power Plant Grid Interconnection Tender"
          className="mt-1 min-h-11 w-full rounded-xl border border-slate-300 bg-white p-3 text-sm dark:border-slate-700 dark:bg-slate-800"
        />
      </ValidatedLabel>

      <p className="text-xs text-slate-500 dark:text-slate-400">
        Save the tender name to open its basic details.
      </p>

      {error && (
        <p role="alert" className="text-xs font-semibold text-red-700 dark:text-red-400">
          {error}
        </p>
      )}

      <div className="flex justify-end pt-2">
        <button className={primary} disabled={busy || !name.trim()}>
          {busy ? "Creating…" : locked ? "Retry same request" : "Create"}
        </button>
      </div>
    </ValidatedForm>
  );
}

function EditDraft({
  draft,
  saved,
  onCancel,
}: {
  draft: Draft;
  saved: (draft: Draft) => void;
  onCancel?: () => void;
}) {
  const [name, setName] = useState(draft.name);
  const [details, setDetails] = useState<BasicDetails>(draft.basic_details ?? {});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  return (
    <ValidatedForm
      className="space-y-4"
      onSubmit={async (e) => {
        e.preventDefault();
        if (busy) return;
        const trimmed = name.trim();
        if (trimmed.length < 2) {
          setError("Tender name must be at least 2 characters.");
          return;
        }
        setBusy(true);
        setError("");
        try {
          const updated = await api<Draft>(`drafts/${draft.id}`, {
            method: "PATCH",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ name: trimmed, basic_details: details }),
          });
          saved(updated);
        } catch (e) {
          setError(failure(e));
        } finally {
          setBusy(false);
        }
      }}
    >
      <ValidatedLabel
        required
        htmlFor="edit-draft-name"
        className="text-xs font-bold text-slate-700 dark:text-slate-300"
      >
        Tender Name
        <ValidatedInput
          id="edit-draft-name"
          name="name"
          required
          minLength={2}
          maxLength={255}
          value={name}
          disabled={busy}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. 50 MW Solar Power Plant Grid Interconnection Tender"
          className="mt-1 min-h-11 w-full rounded-xl border border-slate-300 bg-white p-3 text-sm dark:border-slate-700 dark:bg-slate-800"
        />
      </ValidatedLabel>

      <div className="grid gap-4 sm:grid-cols-2">
        {basicFields.map(field => (
          <ValidatedLabel key={field.key} className="text-xs font-bold text-slate-700 dark:text-slate-300">
            {field.label}
            <ValidatedInput
              name={field.key}
              type={field.type}
              maxLength={field.maxLength}
              value={details[field.key] ?? ""}
              disabled={busy}
              onChange={event => setDetails(previous => ({ ...previous, [field.key]: event.target.value }))}
              className="mt-1 min-h-11 w-full rounded-xl border border-slate-300 bg-white p-3 text-sm dark:border-slate-700 dark:bg-slate-800"
            />
          </ValidatedLabel>
        ))}
      </div>

      {error && (
        <p role="alert" className="text-xs font-semibold text-red-700 dark:text-red-400">
          {error}
        </p>
      )}

      <div className="flex justify-end gap-3 pt-3">
        {onCancel && <button
          type="button"
          className={secondary}
          disabled={busy}
          onClick={onCancel}
        >
          Cancel
        </button>}
        <button
          type="submit"
          className={primary}
          disabled={busy || !name.trim() || (name.trim() === draft.name && basicFields.every(field => (details[field.key] ?? "") === (draft.basic_details?.[field.key] ?? "")))}
        >
          {busy ? "Saving…" : "Save changes"}
        </button>
      </div>
    </ValidatedForm>
  );
}

function DeleteDraft({
  draft,
  deleted,
  onCancel,
}: {
  draft: Draft;
  deleted: () => void;
  onCancel: () => void;
}) {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  return (
    <ValidatedForm
      className="space-y-4"
      onSubmit={async (e) => {
        e.preventDefault();
        if (busy) return;
        setBusy(true);
        setError("");
        try {
          await api(`drafts/${draft.id}`, { method: "DELETE" });
          deleted();
        } catch (e) {
          setError(failure(e));
        } finally {
          setBusy(false);
        }
      }}
    >
      <div className="rounded-xl border border-red-200 bg-red-50/50 p-4 dark:border-red-900/50 dark:bg-red-950/20">
        <p className="text-sm font-semibold text-slate-900 dark:text-white">
          Remove tender from active list?
        </p>
        <p className="mt-1 text-xs text-slate-600 dark:text-slate-400">
          <span className="font-bold">{draft.name}</span> will be soft-deleted and removed from the active grid, while preserving intake records in the database.
        </p>
      </div>

      {error && (
        <p role="alert" className="text-xs font-semibold text-red-700 dark:text-red-400">
          {error}
        </p>
      )}

      <div className="flex justify-end gap-3 pt-3">
        <button
          type="button"
          className={secondary}
          disabled={busy}
          onClick={onCancel}
        >
          Cancel
        </button>
        <button
          type="submit"
          className="inline-flex min-h-11 items-center justify-center rounded-xl bg-red-600 px-4 text-sm font-bold text-white hover:bg-red-700 disabled:opacity-50"
          disabled={busy}
        >
          {busy ? "Deleting…" : "Delete tender"}
        </button>
      </div>
    </ValidatedForm>
  );
}

const workspaceStages: readonly Stage[] = [
  "details", "documents", "tsc", "queries", "replies", "bidding", "boq", "quotes", "costing", "award",
];

function DraftDetail({
  draft,
  canWrite,
  initialStage,
  activeStep,
  onSelectStep,
  onDone,
  onDirty,
  onBeforeLeave,
}: {
  draft: Draft;
  canWrite: boolean;
  initialStage: Stage;
  activeStep?: string;
  onDone?: () => void;
  onDirty?: (value: boolean) => void;
  onBeforeLeave?: () => boolean;
  onSelectStep?: (step: string) => void;
}) {
  const [data, setData] = useState(draft);
  const [error, setError] = useState("");
  const [artifact, setArtifact] = useState<{ title: string; text: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [stage, setStage] = useState(initialStage);
  const [native, setNative] = useState(draft.engine === "nextjs");
  const stageHeading = useRef<HTMLHeadingElement>(null);

  const [previousInitialStage, setPreviousInitialStage] = useState(initialStage);
  if (initialStage !== previousInitialStage) {
    setPreviousInitialStage(initialStage);
    setStage(initialStage);
  }

  useEffect(() => {
    if (stage === "bidding" || stage === "tsc") {
      window.scrollTo(0, 0);
      stageHeading.current?.focus({ preventScroll: true });
    }
  }, [stage]);

  useEffect(() => {
    const c = new AbortController();
    api<{ engine?: string }>(`drafts/${draft.id}/tsc/options`, {
      signal: c.signal,
    })
      .then((o) => setNative(o.engine === "nextjs"))
      .catch(() => {});
    return () => c.abort();
  }, [draft.id]);

  function changeStage(value: Stage) {
    if (value !== stage && onBeforeLeave && !onBeforeLeave()) return;
    setStage(value);
    onSelectStep?.(stepForStage(value));
  }

  const refresh = useCallback(
    async (signal?: AbortSignal) => {
      try {
        const d = await api<Draft>(`drafts/${draft.id}`, { signal });
        setData(d);
        if (d.engine === "nextjs") setNative(true);
        setError("");
      } catch (e) {
        if (!signal?.aborted) setError(failure(e));
      }
    },
    [draft.id]
  );

  useEffect(() => {
    const c = new AbortController();
    let alive = true;
    let timer: ReturnType<typeof setTimeout>;
    async function poll() {
      await refresh(c.signal);
      if (alive) timer = setTimeout(() => void poll(), 3000);
    }
    timer = setTimeout(() => void poll(), 0);
    return () => {
      alive = false;
      c.abort();
      clearTimeout(timer);
    };
  }, [refresh]);

  const verifiedBatches = data.batches.filter(
    (b) => b.state === "consumed" || b.state === "ready"
  ).length;
  const totalBatches = data.batches.length;

  const currentIndex = workspaceStages.indexOf(stage);
  const currentCode = activeStep && stageForStep(activeStep) === stage ? activeStep : stepForStage(stage);
  const workStep = tenderProcess.steps.find(item => item.code === currentCode);
  const prevStage = workspaceStages[currentIndex - 1];
  const hasVerifiedDocuments = Boolean(data.has_verified_batches || verifiedBatches);
  const tscNextStage = data.prebid_review_closed ? "bidding" : "queries";
  const nextStage = stage === "tsc" ? tscNextStage
    : stage === "replies" && data.prebid_review_closed && !data.tsc_completed ? "tsc"
    : workspaceStages[currentIndex + 1];
  const canContinue = stage === "documents" ? hasVerifiedDocuments
    : stage === "replies" ? data.prebid_review_closed
    : stage === "tsc" ? data.tsc_completed
    : stage === "bidding" ? data.bidding_authorized : true;

  return (
    <div className="space-y-6">
      {/* Dedicated Step Header Card */}
      <div className="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="flex items-center gap-3">
          {onDone && (
            <button
              type="button"
              className={secondary + " gap-1.5 px-3 py-1.5 text-xs font-bold"}
              onClick={onDone}
              title="Return to tenders list"
            >
              <ArrowLeftIcon className="h-4 w-4" />
              All tenders
            </button>
          )}

          <div>
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-xs font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
                {workStep ? `Step ${workStep.number} · ${workStep.label}` : stepName(currentCode)}
              </span>
              {totalBatches > 0 && (
                <span
                  className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[11px] font-semibold ${
                    verifiedBatches === totalBatches
                      ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"
                      : "bg-amber-50 text-amber-700 dark:bg-amber-950 dark:text-amber-300"
                  }`}
                >
                  <DocumentCheckIcon className="h-3.5 w-3.5" />
                  {verifiedBatches}/{totalBatches} batches verified
                </span>
              )}
            </div>
            <h1 ref={stageHeading} tabIndex={-1} className="mt-1 text-2xl font-bold text-slate-900 dark:text-white">
              {data.name}
            </h1>
          </div>
        </div>

      </div>

      {stage === "details" ? (
        <section className="rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
          <h2 className="mb-4 text-lg font-semibold">Tender details</h2>
          {canWrite ? (
            <EditDraft draft={data} saved={updated => { setData(updated); void refresh(); }} />
          ) : (
            <dl className="grid gap-4 sm:grid-cols-2">
              {basicFields.map(field => <div key={field.key}><dt>{field.label}</dt><dd>{data.basic_details?.[field.key] || "—"}</dd></div>)}
            </dl>
          )}
        </section>
      ) : stage === "documents" ? (
        <TenderDocumentIntake
          draft={data}
          canWrite={canWrite}
          native={native}
          onRefresh={refresh}
          onViewArtifact={(title, text) => setArtifact({ title, text })}
        />
      ) : stage === "tsc" ? (
        hasVerifiedDocuments || data.tsc_completed || data.prebid_review_closed ? (
          <CloudTSCWorkspace
            onDone={onDone}
            onDirty={onDirty}
            nextStep={{
              label: stepName(stepForStage(tscNextStage)),
              navigate: () => changeStage(tscNextStage),
            }}
            draftID={data.id}
            sourceVersion={data.version}
            readyForGeneration={
              data.batches.length > 0 &&
              data.batches.every(
                (b) =>
                  b.state === "consumed" &&
                  b.sources.length > 0 &&
                  b.sources.every((s) => s.status === "ready")
              )
            }
          />
        ) : (
          <section className="space-y-3 rounded-xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
            <p>Verify the tender documents in Step 2 before preparing the TSC.</p>
            <button type="button" className={primary} onClick={() => changeStage("documents")}>Read &amp; verify documents</button>
          </section>
        )
      ) : stage === "bidding" ? (
        <TenderBiddingApproval
          draftId={data.id}
          canWrite={canWrite}
          intakeDueDate={data.basic_details?.due_date}
          onNavigateStep={(st) => changeStage(stageForStep(st))}
          onDone={onDone}
        />
      ) : stage === "queries" || stage === "replies" ? (
        <TenderPrebidQueries
          reviewing={stage === "replies"}
          closed={Boolean(data.prebid_review_closed)}
          tscCompleted={Boolean(data.tsc_completed)}
          onRefresh={refresh}
          draftId={data.id}
          canWrite={canWrite}
          onNavigateStep={(st) => changeStage(stageForStep(st))}
        />
      ) : stage === "boq" ? (
        <TenderBOQItems
          draftId={data.id}
          canWrite={canWrite}
          onNavigateStep={(st) => changeStage(stageForStep(st))}
        />
      ) : stage === "quotes" ? (
        <TenderVendorComparison
          draftId={data.id}
          canWrite={canWrite}
          onNavigateStep={(st) => changeStage(stageForStep(st))}
        />
      ) : stage === "costing" ? (
        <TenderBidCosting
          draftId={data.id}
          canWrite={canWrite}
          onNavigateStep={(st) => changeStage(stageForStep(st))}
        />
      ) : (
        <TenderAwardHandoff
          draftId={data.id}
          canWrite={canWrite}
        />
      )}

      {/* Bottom Step Progression Navigation Bar */}
      <div className="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div>
          {prevStage ? (
            <button
              type="button"
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-slate-300 bg-slate-100/90 px-4 py-2 text-xs font-bold text-slate-700 transition hover:bg-slate-200 hover:text-slate-900 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-750"
              onClick={() => changeStage(prevStage)}
            >
              <ArrowLeftIcon className="h-4 w-4" />
              Previous · {stepName(stepForStage(prevStage))}
            </button>
          ) : (
            <button
              type="button"
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-slate-300 bg-slate-100/90 px-4 py-2 text-xs font-bold text-slate-700 transition hover:bg-slate-200 hover:text-slate-900 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-750"
              onClick={onDone}
            >
              <ArrowLeftIcon className="h-4 w-4" />
              Back to All Tenders
            </button>
          )}
        </div>

        <div>
          {nextStage ? (
            <button
              type="button"
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-5 py-2 text-xs font-bold text-white shadow-sm transition hover:bg-blue-800 active:scale-[0.99] disabled:opacity-50 dark:bg-blue-600 dark:hover:bg-blue-500"
              disabled={!canContinue}
              onClick={() => changeStage(nextStage)}
            >
              Continue · {stepName(stepForStage(nextStage))}
              <ArrowRightIcon className="h-4 w-4" />
            </button>
          ) : (
            <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-emerald-700 dark:text-emerald-400">
              <CheckCircleIcon className="h-4 w-4" />
              Process Flow Completed
            </span>
          )}
        </div>
      </div>

      {artifact && (
        <RecordDrawer
          title={artifact.title}
          onClose={() => setArtifact(null)}
        >
          <pre className="whitespace-pre-wrap break-words text-xs">{artifact.text}</pre>
        </RecordDrawer>
      )}
    </div>
  );
}
