"use client";
import { TenderProcessingQueue } from "./tender-processing-queue";
import { TenderShell } from "./tender-shell";
import { useEffect, useState, type FormEvent } from "react";
import {
  TenderPdfUpload,
  TenderPdfSource,
} from "@/components/tenders/tender-pdf-intake";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { DataGrid, type GridColumn } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import {
  ValidatedForm,
  ValidatedInput,
  ValidatedLabel,
  ValidatedSelect,
  ValidatedTextarea,
} from "@/components/ui/validated-controls";

type Tender = {
  id: string;
  code: string;
  title: string;
  issuer: string;
  tender_reference: string;
  lot_reference: string;
  location: string;
  submission_date: string | null;
  notes: string;
  owner_id: string | null;
  is_active: boolean;
  version: number;
  created_at: string;
  created_by_id: string | null;
  updated_at: string;
  updated_by_id: string | null;
};
type History = {
  id: string;
  action: string;
  reason: string;
  actor_id: string | null;
  occurred_at: string;
  before: Record<string, unknown>;
  after: Record<string, unknown>;
};
type Draft = Partial<Tender> & { reason: string };
const columns: GridColumn<Tender>[] = [
  {
    key: "code",
    label: "Tender",
    value: (row) => row.code,
    className: "tender-code-cell",
  },
  {
    key: "title",
    label: "Title",
    value: (row) => row.title,
    className: "tender-title-cell",
    render: (row) => (
      <span className="tender-title-preview" title={row.title}>
        {row.title}
      </span>
    ),
  },
  {
    key: "issuer",
    label: "Issuing authority",
    value: (row) => row.issuer || "Not recorded",
  },
  {
    key: "reference",
    label: "Reference / lot",
    value: (row) =>
      [row.tender_reference, row.lot_reference].filter(Boolean).join(" / ") ||
      "Not recorded",
  },
  {
    key: "submission",
    label: "Submission date",
    value: (row) => row.submission_date || "Not recorded",
  },
  {
    key: "status",
    label: "Status",
    value: (row) => (row.is_active ? "Received" : "Inactive"),
    render: (row) => (
      <span className={`tender-status ${row.is_active ? "is-active" : ""}`}>
        {row.is_active ? "Received" : "Inactive"}
      </span>
    ),
  },
];
const fieldClass = "field";
function responseError(status: number, fallback: string): string {
  return status === 403
    ? "Your account does not have permission for this action."
    : status === 404
      ? "This tender is unavailable to your account."
      : fallback;
}

export function TenderRegister() {
  const user = useAuthenticatedUser();
  const has = (code: string) =>
    user.permissions.includes("system:admin") ||
    user.permissions.includes(code);
  const canWrite =
    has("tenders.register.write") &&
    has("tenders.processing.write") &&
    has("tenders.processing.view") &&
    has("tenders.processing.read");
  const canViewQueue =
    has("tenders.processing.view") && has("tenders.processing.read");
  const [view, setView] = useState<"register" | "queue">("register");
  const [showUpload, setShowUpload] = useState(false);
  const [rows, setRows] = useState<Tender[]>([]);
  const [total, setTotal] = useState(0);
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [active, setActive] = useState("true");
  const [offset, setOffset] = useState(0);
  const [reload, setReload] = useState(0);
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState("");
  const [notice, setNotice] = useState("");
  const [draft, setDraft] = useState<Draft | null>(null);
  const [editing, setEditing] = useState(false);
  const [opening, setOpening] = useState(false);
  const [saving, setSaving] = useState(false);
  const [panelError, setPanelError] = useState("");
  const [history, setHistory] = useState<History[]>([]);
  const [historyOffset, setHistoryOffset] = useState(0);
  const [historyMore, setHistoryMore] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams({
      q: query,
      offset: String(offset),
      limit: "100",
    });
    if (active) params.set("active", active);
    void fetch(`/api/tenders?${params}`, {
      cache: "no-store",
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok)
          throw new Error(
            responseError(response.status, "Unable to load tenders."),
          );
        return response.json() as Promise<{ items: Tender[]; total: number }>;
      })
      .then((result) => {
        if (controller.signal.aborted) return;
        setRows((previous) =>
          offset === 0
            ? result.items
            : [
                ...previous,
                ...result.items.filter(
                  (row) => !previous.some((existing) => existing.id === row.id),
                ),
              ],
        );
        setTotal(result.total);
      })
      .catch((error) => {
        if (!controller.signal.aborted) setListError(error.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [query, active, offset, reload]);

  const selectedId = draft?.id;
  useEffect(() => {
    if (!selectedId) return;
    const controller = new AbortController();
    void fetch(
      `/api/tenders/${selectedId}/history?offset=${historyOffset}&limit=100`,
      { cache: "no-store", signal: controller.signal },
    )
      .then(async (response) => {
        if (!response.ok)
          throw new Error(
            responseError(response.status, "Unable to load tender history."),
          );
        return response.json() as Promise<History[]>;
      })
      .then((result) => {
        if (!controller.signal.aborted) {
          setHistory((previous) =>
            historyOffset === 0 ? result : [...previous, ...result],
          );
          setHistoryMore(result.length === 100);
        }
      })
      .catch((error) => {
        if (!controller.signal.aborted) setHistoryError(error.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setHistoryLoading(false);
      });
    return () => controller.abort();
  }, [selectedId, historyOffset]);

  async function open(row: Pick<Tender, "id">, edit: boolean) {
    if (opening) return;
    setOpening(true);
    setListError("");
    try {
      const response = await fetch(`/api/tenders/${row.id}`, {
        cache: "no-store",
      });
      if (!response.ok)
        throw new Error(
          responseError(response.status, "Unable to open tender."),
        );
      const record: Tender = await response.json();
      setPanelError("");
      setHistory([]);
      setHistoryOffset(0);
      setHistoryMore(false);
      setHistoryLoading(true);
      setHistoryError("");
      setEditing(edit);
      setDraft({ ...record, reason: "" });
    } catch (error) {
      setListError(
        error instanceof Error ? error.message : "Unable to open tender.",
      );
    } finally {
      setOpening(false);
    }
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!draft || saving) return;
    const form = event.currentTarget;
    setSaving(true);
    setPanelError("");
    const payload = {
      title: draft.title,
      issuer: draft.issuer || "",
      tender_reference: draft.tender_reference || "",
      lot_reference: draft.lot_reference || "",
      location: draft.location || "",
      submission_date: draft.submission_date || null,
      notes: draft.notes || "",
      ...(draft.id
        ? {
            expected_version: draft.version,
            is_active: draft.is_active,
            reason: draft.reason,
          }
        : {}),
    };
    try {
      const response = await fetch(
        `/api/tenders${draft.id ? `/${draft.id}` : ""}`,
        {
          method: draft.id ? "PATCH" : "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
      );
      const result = await response.json();
      if (!response.ok) {
        if (!displayServerErrors(form, result))
          setPanelError(
            responseError(
              response.status,
              typeof result.detail === "string"
                ? result.detail
                : "Unable to save tender.",
            ),
          );
        return;
      }
      setNotice(`${result.code} ${draft.id ? "updated" : "created"}.`);
      setDraft(null);
      setOffset(0);
      setRows([]);
      setTotal(0);
      setLoading(true);
      setListError("");
      setReload((value) => value + 1);
    } catch {
      setPanelError(
        "Unable to reach the server. Your changes are retained; reopen the register if the save outcome is uncertain.",
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <TenderShell
      action={
        canWrite ? (
          <button
            className="tender-primary"
            onClick={() => setShowUpload((value) => !value)}
            aria-expanded={showUpload}
          >
            {showUpload ? "Close upload" : "+ Upload tender PDF"}
          </button>
        ) : null
      }
    >
      <section className="tender-register space-y-4">
        <nav aria-label="Tender views" className="tender-view-tabs">
          <button
            aria-current={view === "register" ? "page" : undefined}
            onClick={() => {
              setView("register");
              setOffset(0);
              setReload((value) => value + 1);
            }}
          >
            Tender register
          </button>
          {canViewQueue ? (
            <button
              aria-current={view === "queue" ? "page" : undefined}
              onClick={() => setView("queue")}
            >
              Processing queue
            </button>
          ) : null}
        </nav>

        {canWrite && showUpload ? (
          <TenderPdfUpload
            onQueued={() => {
              setNotice(
                "Documents queued. Open a completed job to review the extracted tender.",
              );
              setShowUpload(false);
              setView("queue");
            }}
          />
        ) : null}
        {notice ? (
          <p role="status" className="rounded-xl bg-blue-50 p-4 text-blue-800">
            {notice}
          </p>
        ) : null}
        {listError ? (
          <div role="alert" className="rounded-xl bg-red-50 p-4 text-red-800">
            {listError}
            <button
              className="ml-3 min-h-11 font-bold underline"
              onClick={() => {
                setOffset(0);
                setRows([]);
                setTotal(0);
                setLoading(true);
                setListError("");
                setReload((value) => value + 1);
              }}
            >
              Retry
            </button>
          </div>
        ) : null}
        {view === "queue" && canViewQueue ? (
          <TenderProcessingQueue
            onOpen={(id) => void open({ id }, false)}
            onCompleted={() => {
              setOffset(0);
              setReload((value) => value + 1);
            }}
          />
        ) : null}
        <div hidden={view !== "register"} className="space-y-4">
          <ValidatedForm
            onSubmit={(event) => {
              event.preventDefault();
              setOffset(0);
              setRows([]);
              setTotal(0);
              setLoading(true);
              setListError("");
              setQuery(search.trim());
              setReload((value) => value + 1);
            }}
            className="flex flex-wrap items-end gap-3"
          >
            <ValidatedLabel className="text-sm font-semibold">
              Search all accessible tenders
              <ValidatedInput
                name="q"
                maxLength={255}
                className={fieldClass}
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Title, issuer, reference or location"
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-sm font-semibold">
              Register status
              <ValidatedSelect
                className={fieldClass}
                value={active}
                onChange={(event) => {
                  setOffset(0);
                  setRows([]);
                  setTotal(0);
                  setLoading(true);
                  setListError("");
                  setActive(event.target.value);
                }}
              >
                <option value="true">Active</option>
                <option value="false">Inactive</option>
                <option value="">All</option>
              </ValidatedSelect>
            </ValidatedLabel>
            <button
              disabled={loading}
              className="min-h-11 rounded-xl border border-slate-300 bg-white px-4 font-bold"
            >
              Search
            </button>
          </ValidatedForm>
          <DataGrid
            label="Tenders"
            rows={rows}
            columns={columns}
            loading={loading}
            searchable={false}
            initialSort="code"
            onRowClick={(row) => void open(row, false)}
            actions={(row) => (
              <div className="flex justify-end gap-1">
                <button
                  disabled={opening}
                  aria-label={`View tender ${row.code}`}
                  className="min-h-11 rounded-lg px-3 font-bold text-blue-700"
                  onClick={() => void open(row, false)}
                >
                  View
                </button>
                {has("tenders.register.write") &&
                (row.owner_id === user.id || has("tenders.portfolio.write")) ? (
                  <button
                    disabled={opening}
                    aria-label={`Edit tender ${row.code}`}
                    className="min-h-11 rounded-lg px-3 font-bold text-blue-700"
                    onClick={() => void open(row, true)}
                  >
                    Edit
                  </button>
                ) : null}
              </div>
            )}
          />
          <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-slate-600">
            <p role="status">
              {rows.length} of {total} accessible tenders loaded.
            </p>
            {rows.length < total ? (
              <button
                disabled={loading}
                className="min-h-11 rounded-xl border bg-white px-4 font-bold"
                onClick={() => {
                  setLoading(true);
                  setListError("");
                  setOffset((value) => value + 100);
                }}
              >
                Load next 100 tenders
              </button>
            ) : null}
          </div>
        </div>
        {draft ? (
          <RecordDrawer
            title={
              draft.id
                ? `${editing ? "Edit" : "View"} ${draft.code}`
                : "New tender"
            }
            onClose={() => {
              if (!saving) setDraft(null);
            }}
            footer={
              <div className="flex justify-end gap-2">
                <button
                  disabled={saving}
                  className="min-h-11 rounded-xl border px-4"
                  onClick={() => setDraft(null)}
                >
                  {editing ? "Cancel" : "Close"}
                </button>
                {editing ? (
                  <button
                    disabled={saving}
                    type="submit"
                    form="tender-form"
                    className="min-h-11 rounded-xl bg-blue-700 px-4 font-bold text-white"
                  >
                    {saving ? "Saving…" : "Save tender"}
                  </button>
                ) : null}
              </div>
            }
          >
            {panelError ? (
              <p role="alert" className="rounded-xl bg-red-50 p-4 text-red-800">
                {panelError}
              </p>
            ) : null}
            {draft.id ? <TenderPdfSource key={draft.id} id={draft.id} /> : null}
            <ValidatedForm
              id="tender-form"
              onSubmit={(event) => void save(event)}
            >
              <fieldset
                disabled={saving || !editing}
                className="grid gap-4 sm:grid-cols-2"
              >
                <ValidatedLabel className="sm:col-span-2">
                  Tender title
                  <ValidatedInput
                    name="title"
                    required
                    minLength={2}
                    maxLength={2000}
                    className={fieldClass}
                    value={draft.title || ""}
                    onChange={(event) =>
                      setDraft({ ...draft, title: event.target.value })
                    }
                  />
                </ValidatedLabel>
                <ValidatedLabel>
                  Issuing authority
                  <ValidatedInput
                    name="issuer"
                    required={Boolean(draft.tender_reference?.trim())}
                    maxLength={255}
                    className={fieldClass}
                    value={draft.issuer || ""}
                    onChange={(event) =>
                      setDraft({ ...draft, issuer: event.target.value })
                    }
                  />
                </ValidatedLabel>
                <ValidatedLabel>
                  Tender reference
                  <ValidatedInput
                    name="tender_reference"
                    required={Boolean(draft.lot_reference?.trim())}
                    maxLength={128}
                    className={fieldClass}
                    value={draft.tender_reference || ""}
                    onChange={(event) =>
                      setDraft({
                        ...draft,
                        tender_reference: event.target.value,
                      })
                    }
                  />
                </ValidatedLabel>
                <ValidatedLabel>
                  Lot reference
                  <ValidatedInput
                    name="lot_reference"
                    maxLength={128}
                    className={fieldClass}
                    value={draft.lot_reference || ""}
                    onChange={(event) =>
                      setDraft({ ...draft, lot_reference: event.target.value })
                    }
                  />
                </ValidatedLabel>
                <ValidatedLabel>
                  Location
                  <ValidatedInput
                    name="location"
                    maxLength={255}
                    className={fieldClass}
                    value={draft.location || ""}
                    onChange={(event) =>
                      setDraft({ ...draft, location: event.target.value })
                    }
                  />
                </ValidatedLabel>
                <ValidatedLabel>
                  Submission date
                  <ValidatedInput
                    name="submission_date"
                    type="date"
                    min="1900-01-01"
                    max="2200-12-31"
                    className={fieldClass}
                    value={draft.submission_date || ""}
                    onChange={(event) =>
                      setDraft({
                        ...draft,
                        submission_date: event.target.value || null,
                      })
                    }
                  />
                </ValidatedLabel>
                <p className="self-center text-sm text-slate-600">
                  Date only. Submission time and timezone have not been
                  recorded.
                </p>
                <ValidatedLabel className="sm:col-span-2">
                  Notes
                  <ValidatedTextarea
                    name="notes"
                    maxLength={2000}
                    rows={4}
                    className={fieldClass}
                    value={draft.notes || ""}
                    onChange={(event) =>
                      setDraft({ ...draft, notes: event.target.value })
                    }
                  />
                </ValidatedLabel>
                {draft.id ? (
                  <>
                    <ValidatedLabel className="flex min-h-11 items-center gap-2">
                      <ValidatedInput
                        type="checkbox"
                        checked={draft.is_active}
                        onChange={(event) =>
                          setDraft({
                            ...draft,
                            is_active: event.target.checked,
                          })
                        }
                      />
                      Active
                    </ValidatedLabel>
                    {editing ? (
                      <ValidatedLabel className="sm:col-span-2">
                        Change reason
                        <ValidatedTextarea
                          name="reason"
                          required
                          minLength={3}
                          maxLength={1000}
                          rows={2}
                          className={fieldClass}
                          value={draft.reason}
                          onChange={(event) =>
                            setDraft({ ...draft, reason: event.target.value })
                          }
                        />
                      </ValidatedLabel>
                    ) : null}
                  </>
                ) : null}
              </fieldset>
            </ValidatedForm>
            {draft.id ? (
              <section className="space-y-3 border-t border-slate-200 pt-4">
                <h3 className="text-lg font-bold">Change history</h3>
                <p className="text-sm text-slate-600">
                  Version {draft.version} · Created{" "}
                  {new Date(draft.created_at!).toLocaleString()} · Owner{" "}
                  {draft.owner_id || "Unassigned"}
                </p>
                {historyError ? (
                  <p role="alert" className="text-red-700">
                    {historyError}
                  </p>
                ) : null}
                {historyLoading ? <p role="status">Loading history…</p> : null}
                {!historyLoading && !history.length && !historyError ? (
                  <p>No changes recorded.</p>
                ) : null}
                <ol className="space-y-3">
                  {history.map((event) => (
                    <li
                      key={event.id}
                      className="rounded-xl border border-slate-200 bg-white p-3"
                    >
                      <p className="font-bold capitalize">
                        {event.action} ·{" "}
                        {new Date(event.occurred_at).toLocaleString()}
                      </p>
                      <p className="mt-1 text-sm">{event.reason}</p>
                      <p className="mt-1 break-all text-xs text-slate-600">
                        Actor: {event.actor_id || "Legacy / unavailable"}
                      </p>
                      <details className="mt-2 text-sm">
                        <summary className="cursor-pointer font-semibold text-blue-700">
                          View recorded changes
                        </summary>
                        <dl className="mt-2 space-y-2">
                          {Object.keys(event.after)
                            .filter(
                              (key) =>
                                JSON.stringify(event.before[key]) !==
                                JSON.stringify(event.after[key]),
                            )
                            .map((key) => (
                              <div key={key}>
                                <dt className="font-semibold">
                                  {key.replaceAll("_", " ")}
                                </dt>
                                <dd className="whitespace-pre-wrap break-words">
                                  {String(event.before[key] ?? "Not recorded")}{" "}
                                  → {String(event.after[key] ?? "Not recorded")}
                                </dd>
                              </div>
                            ))}
                        </dl>
                      </details>
                    </li>
                  ))}
                </ol>
                {historyMore ? (
                  <button
                    disabled={historyLoading}
                    className="min-h-11 rounded-lg border px-3 font-semibold"
                    onClick={() => {
                      setHistoryLoading(true);
                      setHistoryError("");
                      setHistoryOffset((value) => value + 100);
                    }}
                  >
                    Load older history
                  </button>
                ) : null}
              </section>
            ) : null}
          </RecordDrawer>
        ) : null}
      </section>
    </TenderShell>
  );
}
