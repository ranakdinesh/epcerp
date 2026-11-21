"use client";

import { useState, useRef } from "react";
import { fileSHA256 } from "@/lib/file-sha256";
import { createRecordId } from "@/lib/record-id";
import {
  CheckCircleIcon,
  DocumentCheckIcon,
  EyeIcon,
  ArrowPathIcon,
} from "@heroicons/react/24/outline";
import { DataGrid } from "@/components/ui/data-grid";
import {
  ValidatedLabel,
  ValidatedSelect,
} from "@/components/ui/validated-controls";
import { packageFileError, type Receipt } from "./tender-capture-model";
import { TenderDocumentList, type TenderDocument } from "./tender-document-list";
import { TenderUploadSelection } from "./tender-upload-selection";

export type Source = {
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

export type Batch = {
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

export type Draft = {
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
};

const primary =
  "inline-flex min-h-10 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-xs font-bold text-white shadow-sm transition hover:bg-blue-800 disabled:opacity-50";
const secondary =
  "inline-flex min-h-10 items-center justify-center gap-2 rounded-xl border border-slate-300 bg-white px-3 text-xs font-semibold text-slate-700 shadow-2xs transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-750 disabled:opacity-50";
const icon =
  "inline-flex min-h-10 min-w-10 items-center justify-center rounded-xl text-blue-700 transition hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-slate-800 disabled:opacity-50";

class ApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
  }
}

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(`/api/tender-fms/${path}`, { cache: "no-store", ...init });
  const body = await r.json().catch(() => ({ detail: "Request could not be processed" }));
  if (!r.ok) throw new ApiError(body.detail || `Request failed (${r.status})`, r.status);
  return body as T;
}

const json = (body: unknown, key?: string): RequestInit => ({
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    ...(key ? { "Idempotency-Key": key } : {}),
  },
  body: JSON.stringify(body),
});

function failure(e: unknown): string {
  return e instanceof Error ? e.message : "Service temporarily unavailable";
}

export function TenderDocumentIntake({
  draft,
  canWrite,
  native,
  onRefresh,
  onViewArtifact,
}: {
  draft: Draft;
  canWrite: boolean;
  native: boolean;
  onRefresh: () => Promise<void> | void;
  onViewArtifact?: (title: string, text: string) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const verifiedBatches = draft.batches.filter(
    (b) => b.state === "consumed" || b.state === "ready"
  ).length;
  const totalBatches = draft.batches.length;
  const attachedDocs = draft.batches.flatMap((b) => b.documents ?? []);
  const isVerifying = draft.batches.some(
    (b) => b.state === "verifying" || b.state === "receiving"
  );

  const sources = draft.batches.flatMap((b) =>
    (b.sources ?? []).map((s) => ({
      ...s,
      occurrence_id: `${b.revision}:${s.path}`,
      revision: b.revision,
    }))
  );
  const read = sources.filter((s) => s.status === "ready").length;

  async function view(s: Source, kind?: "text" | "history") {
    setBusy(true);
    try {
      if (kind === "history") {
        const h = await api<{ history: { attempts: number; warnings: string[] } }>(
          `drafts/${draft.id}/documents/${s.id}/history`
        );
        onViewArtifact?.(
          `Document History · ${s.filename}`,
          JSON.stringify(h, null, 2)
        );
      } else {
        const res = await api<{ text: string }>(
          `drafts/${draft.id}/documents/${s.id}/result`
        );
        onViewArtifact?.(`Extracted Text · ${s.filename}`, res.text);
      }
    } catch (e) {
      setError(failure(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-6">
      {/* Verification In-Progress Banner */}
      {isVerifying && (
        <div className="flex items-center gap-3 rounded-2xl border border-blue-200 bg-blue-50/70 p-4 text-xs font-semibold text-blue-900 dark:border-blue-900/50 dark:bg-blue-950/40 dark:text-blue-200">
          <ArrowPathIcon className="h-4 w-4 shrink-0 animate-spin text-blue-600 dark:text-blue-400" />
          <span>Expanding and verifying uploaded tender documents… Extracted files will appear below automatically.</span>
        </div>
      )}

      {error && (
        <p role="alert" className="text-xs font-semibold text-red-700 dark:text-red-400">
          {error}
        </p>
      )}

      {/* Upload Zone */}
      {canWrite && <UploadBatch draft={draft} changed={() => void onRefresh()} />}

      {/* Verified Attached Documents List */}
      {native && attachedDocs.length > 0 && (
        <TenderDocumentList
          draftID={draft.id}
          version={draft.version}
          documents={attachedDocs}
          canWrite={canWrite}
          changed={async () => {
            await onRefresh();
          }}
        />
      )}

      {/* Batches Log */}
      {draft.batches.length > 0 && (
        <div className="space-y-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <h3 className="text-sm font-bold text-slate-900 dark:text-white">
            Upload & Extraction Batches ({draft.batches.length})
          </h3>
          <div className="space-y-2">
            {draft.batches.map((b) => (
              <div
                key={b.id}
                className="rounded-xl border border-slate-100 bg-slate-50/70 p-3 text-xs dark:border-slate-800 dark:bg-slate-850"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="font-semibold text-slate-900 dark:text-white">
                    Batch {b.revision.slice(0, 8)}:{" "}
                    <span className="font-normal text-slate-600 dark:text-slate-300">
                      {b.state === "consumed"
                        ? native
                          ? "Verified and attached"
                          : "Reading jobs published"
                        : b.state.replaceAll("_", " ")}
                    </span>
                  </p>
                  <span className="text-[11px] text-slate-500">
                    Attempts: {b.attempts}/3
                  </span>
                </div>

                <ul className="mt-1.5 space-y-1">
                  {b.files.map((f) => (
                    <li key={f.id} className="text-slate-600 dark:text-slate-300">
                      • {f.name} (
                      {native && f.kind === "zip" && (b.state === "ready" || b.state === "consumed")
                        ? f.archive_deleted_at
                          ? "extracted · ZIP deleted"
                          : "extracted · deleting ZIP"
                        : f.state}
                      )
                      {!(native && f.kind === "zip" && (b.state === "ready" || b.state === "consumed")) &&
                      f.state !== "declared" &&
                      f.state !== "uploading" ? (
                        <a
                          className="ml-2 font-semibold text-blue-700 underline dark:text-blue-400"
                          href={`/api/tender-fms/receipts/${b.id}/files/${f.id}`}
                          download={f.name}
                        >
                          Download original
                        </a>
                      ) : null}
                    </li>
                  ))}
                </ul>

                {b.error && (
                  <p role="alert" className="mt-1.5 font-semibold text-red-600 dark:text-red-400">
                    {b.error.replaceAll("_", " ")}.{" "}
                    {b.state === "verifying"
                      ? "A bounded retry is scheduled."
                      : "Batch blocked. Its original upload is retained so verification can be retried."}
                  </p>
                )}

                {canWrite && b.state === "failed" && b.attempts < 3 && (
                  <div className="mt-2">
                    <button
                      type="button"
                      className={secondary}
                      disabled={busy}
                      onClick={async () => {
                        setBusy(true);
                        try {
                          await api(
                            `receipts/${b.id}/seal`,
                            json({ expected_version: b.version }, createRecordId())
                          );
                          await onRefresh();
                        } catch (e) {
                          setError(failure(e));
                        } finally {
                          setBusy(false);
                        }
                      }}
                    >
                      Retry verification
                    </button>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Coverage Table for Non-Native or Raw Sources */}
      {!native && (
        <DataGrid
          stateKey="tender_sources"
          label="Source reading coverage"
          rows={sources.map((s) => ({ ...s, id: s.occurrence_id }))}
          searchable
          columns={[
            {
              key: "path",
              label: "Source / ZIP path",
              value: (s) => `Batch ${s.revision.slice(0, 8)}: ${s.path}`,
            },
            { key: "status", label: "Reading", value: (s) => s.status },
            {
              key: "coverage",
              label: "Coverage",
              value: (s) =>
                `${s.segments} segments${
                  s.expected_pages
                    ? ` / ${s.expected_pages} expected pages`
                    : s.expected_sheets
                    ? ` / ${s.expected_sheets} expected sheets`
                    : ""
                }`,
            },
            { key: "attempts", label: "Attempts", value: (s) => s.attempts },
            {
              key: "updated",
              label: "Last progress",
              value: (s) => s.updated_at,
              render: (s) => new Date(s.updated_at).toLocaleString(),
            },
          ]}
          actions={(row) => {
            const s = sources.find((src) => src.member_id === row.occurrence_id)!;
            return (
              <>
                <button
                  className={icon}
                  disabled={busy || s.status !== "ready"}
                  title="Read extracted text"
                  aria-label={`Read ${s.path}`}
                  onClick={() => void view(s)}
                >
                  <EyeIcon className="h-5 w-5" />
                </button>
                <button className={secondary} onClick={() => void view(s, "history")}>
                  History
                </button>
                <a
                  className={secondary}
                  href={`/api/tender-fms/drafts/${draft.id}/documents/${s.id}/result`}
                  target="_blank"
                  rel="noreferrer"
                >
                  Evidence JSON
                </a>
                <a
                  className={secondary}
                  href={`/api/tender-fms/drafts/${draft.id}/documents/${s.id}/original`}
                  download={s.filename}
                >
                  Original
                </a>
              </>
            );
          }}
        />
      )}
    </div>
  );
}

function UploadBatch({
  draft,
  changed,
}: {
  draft: Draft;
  changed: () => void;
}) {
  const [files, setFiles] = useState<File[]>([]);
  const [receiptID, setReceiptID] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [fileError, setFileError] = useState("");
  const [progress, setProgress] = useState("");
  const [batchLocked, setBatchLocked] = useState(false);

  const plan = useRef<{
    key: string;
    seal: string;
    declarations: {
      client_file_key: string;
      filename: string;
      kind: string;
      size_bytes: number;
      sha256: string;
    }[];
    uploads: Record<string, string>;
  } | null>(null);

  async function upload() {
    if (busy) return;
    const invalid = packageFileError(files);
    setFileError(invalid);
    if (invalid) return;
    setError("");
    setBusy(true);
    try {
      if (!plan.current) {
        const declarations = [];
        for (const file of files) {
          setProgress(`Checking ${file.name}`);
          const digest = await fileSHA256(file);
          declarations.push({
            client_file_key: createRecordId(),
            filename: file.name,
            kind: file.name.split(".").at(-1)!.toLowerCase(),
            size_bytes: file.size,
            sha256: digest,
          });
        }
        plan.current = {
          key: createRecordId(),
          seal: createRecordId(),
          declarations,
          uploads: {},
        };
      }
      const p = plan.current;
      let receipt = receiptID
        ? await api<Receipt>(`receipts/${receiptID}`)
        : await api<Receipt>("receipts", json({ files: p.declarations }, p.key));
      setReceiptID(receipt.id);
      if (
        receiptID &&
        files.some((file) => !receipt.files.some((saved) => saved.filename === file.name))
      ) {
        throw new Error(
          "This existing batch has a fixed file list. Select its original files; upload additional documents as a new batch afterwards."
        );
      }
      setBatchLocked(true);
      await api(`drafts/${draft.id}/batches`, json({ receipt_id: receipt.id }));
      changed();
      if (receipt.state === "receiving") {
        for (const f of receipt.files) {
          if (f.state === "received" || f.state === "ready") continue;
          const file = files.find((v) => v.name === f.filename);
          if (!file) throw new Error(`Reselect the original file ${f.filename}.`);
          setProgress(`Uploading ${file.name}`);
          p.uploads[f.file_id] ??= createRecordId();
          receipt = await api<Receipt>(
            `receipts/${receipt.id}/files/${f.file_id}`,
            {
              method: "PUT",
              headers: {
                "Content-Type": "application/octet-stream",
                "Idempotency-Key": p.uploads[f.file_id],
                "X-Upload-Version": String(f.version),
              },
              body: file,
            }
          );
        }
      }
      if (receipt.state === "receiving" || receipt.state === "failed") {
        await api(
          `receipts/${receipt.id}/seal`,
          json({ expected_version: receipt.version }, p.seal)
        );
      }
      setProgress(
        "Uploaded. Extracting and verifying documents. The ZIP will be deleted after successful extraction."
      );
      setFiles([]);
      setReceiptID("");
      setBatchLocked(false);
      plan.current = null;
      changed();
    } catch (e) {
      setProgress("");
      setError(failure(e));
      changed();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <TenderUploadSelection
        files={files}
        changed={(selected) => {
          setFiles(selected);
          setError("");
          setProgress("");
          plan.current = null;
          setBatchLocked(false);
        }}
        error={fileError}
        setError={setFileError}
        disabled={busy}
        locked={batchLocked}
      />

      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-slate-100 pt-3 dark:border-slate-800">
        <p className="text-xs text-slate-500 dark:text-slate-400">
          Select Upload when the list is ready. You can add more documents afterwards.
        </p>

        <div className="flex flex-wrap items-center gap-3">
          {draft.batches.some((b) => b.state === "receiving" || b.state === "failed") && (
            <div className="flex items-center gap-2 text-xs">
              <ValidatedLabel
                htmlFor="resume-batch"
                className="font-semibold text-slate-700 dark:text-slate-300"
              >
                Resume batch:
              </ValidatedLabel>
              <ValidatedSelect
                id="resume-batch"
                className="min-h-9 rounded-xl border border-slate-300 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800"
                disabled={busy}
                value={receiptID}
                onChange={(e) => {
                  setReceiptID(e.target.value);
                  plan.current = null;
                  setBatchLocked(false);
                  setError("");
                }}
              >
                <option value="">New upload batch</option>
                {draft.batches
                  .filter((b) => b.state === "receiving" || b.state === "failed")
                  .map((b) => (
                    <option key={b.id} value={b.id}>
                      Batch {b.revision.slice(0, 8)} ({b.state})
                    </option>
                  ))}
              </ValidatedSelect>
            </div>
          )}

          <button
            type="button"
            className={primary}
            disabled={busy || !files.length}
            onClick={() => void upload()}
          >
            {busy ? "Uploading…" : error && batchLocked ? "Retry upload" : "Upload and verify"}
          </button>
        </div>
      </div>

      {error && (
        <p role="alert" className="text-xs font-semibold text-red-600 dark:text-red-400">
          {error}
        </p>
      )}
      {progress && (
        <p role="status" className="text-xs font-medium text-blue-700 dark:text-blue-300">
          {progress}
        </p>
      )}
    </div>
  );
}
