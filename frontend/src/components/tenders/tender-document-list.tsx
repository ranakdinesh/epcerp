"use client";

import { useState } from "react";
import {
  DocumentTextIcon,
  TrashIcon,
  ArrowDownTrayIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
} from "@heroicons/react/24/outline";

export type TenderDocument = {
  id: string;
  filename: string;
  path: string;
  kind: string;
  byte_size: number;
  pages: number;
};

const actionBtn =
  "inline-flex min-h-9 items-center justify-center gap-1.5 rounded-xl border px-3 text-xs font-bold transition disabled:opacity-50";

export function TenderDocumentList({
  draftID,
  version,
  documents,
  canWrite,
  changed,
}: {
  draftID: string;
  version: number;
  documents: TenderDocument[];
  canWrite: boolean;
  changed: () => Promise<void>;
}) {
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  async function remove(document: TenderDocument) {
    if (busy) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch(
        `/api/tender-fms/drafts/${draftID}/documents/${document.id}`,
        {
          method: "DELETE",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ expected_version: version }),
        }
      );
      if (!response.ok) {
        throw Error(
          (await response.json()).detail || "Unable to delete document"
        );
      }
      await changed();
      setConfirm("");
      setMessage(`${document.filename} deleted from this tender.`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to delete document");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section
      aria-label="Tender documents"
      className="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <h2 className="text-base font-bold text-slate-900 dark:text-white">
            Attached Tender Documents
          </h2>
          <span className="rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-bold text-blue-800 dark:bg-blue-950 dark:text-blue-300">
            {documents.length}
          </span>
        </div>
        <p className="text-xs text-slate-500 dark:text-slate-400">
          Verified documents are available in Step 2 for AI & manual evaluation.
        </p>
      </div>

      {message && (
        <div className="flex items-center gap-2 rounded-xl bg-emerald-50 p-3 text-xs font-semibold text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300">
          <CheckCircleIcon className="h-4 w-4 shrink-0 text-emerald-600" />
          <span>{message}</span>
        </div>
      )}

      {error && (
        <div className="flex items-center gap-2 rounded-xl bg-red-50 p-3 text-xs font-semibold text-red-800 dark:bg-red-950/60 dark:text-red-300">
          <ExclamationTriangleIcon className="h-4 w-4 shrink-0 text-red-600" />
          <span>{error}</span>
        </div>
      )}

      {documents.length > 0 ? (
        <div className="overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
          <table className="w-full text-left text-xs">
            <thead className="border-b border-slate-200 bg-slate-50/75 text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:border-slate-800 dark:bg-slate-800/50 dark:text-slate-400">
              <tr>
                <th className="py-2.5 pl-4 pr-3">Document</th>
                <th className="px-3 py-2.5">Type & Size</th>
                <th className="px-3 py-2.5">TSC Readiness</th>
                <th className="py-2.5 pl-3 pr-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
              {documents.map((doc) => {
                const isPdf = doc.kind === "pdf";
                return (
                  <tr
                    key={doc.id}
                    className="transition hover:bg-slate-50/75 dark:hover:bg-slate-850"
                  >
                    <td className="py-3 pl-4 pr-3">
                      <div className="flex items-start gap-3">
                        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                          <DocumentTextIcon className="h-4 w-4" />
                        </div>
                        <div className="min-w-0">
                          <p className="truncate font-semibold text-slate-900 dark:text-white">
                            {doc.filename}
                          </p>
                          <p className="truncate text-[11px] text-slate-500 dark:text-slate-400">
                            {doc.path}
                          </p>
                        </div>
                      </div>
                    </td>

                    <td className="px-3 py-3">
                      <div className="flex items-center gap-1.5">
                        <span className="rounded bg-slate-100 px-2 py-0.5 text-[10px] font-bold text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                          {doc.kind.toUpperCase()}
                        </span>
                        <span className="text-slate-600 dark:text-slate-400">
                          {(doc.byte_size / 1048576).toFixed(2)} MiB
                        </span>
                        {isPdf && doc.pages > 0 && (
                          <span className="text-slate-400">
                            · {doc.pages} pgs
                          </span>
                        )}
                      </div>
                    </td>

                    <td className="px-3 py-3">
                      {isPdf ? (
                        <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                          <span className="h-1.5 w-1.5 rounded-full bg-emerald-600" />
                          Ready for AI Scrutiny
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-semibold text-amber-700 dark:bg-amber-950 dark:text-amber-300">
                          Reference · Export to PDF
                        </span>
                      )}
                    </td>

                    <td className="py-3 pl-3 pr-4 text-right">
                      <div className="flex items-center justify-end gap-1.5">
                        <a
                          className={`${actionBtn} border-slate-200 bg-white text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700`}
                          href={`/api/tender-fms/drafts/${draftID}/documents/${doc.id}/original`}
                          download={doc.filename}
                          title="Download document"
                        >
                          <ArrowDownTrayIcon className="h-3.5 w-3.5" />
                          Download
                        </a>

                        {canWrite && (
                          <button
                            type="button"
                            disabled={busy}
                            className={`${actionBtn} border-red-200 bg-white text-red-700 hover:bg-red-50 dark:border-red-900/60 dark:bg-slate-800 dark:text-red-400 dark:hover:bg-red-950/40`}
                            aria-label={`Delete ${doc.filename}`}
                            title="Delete document"
                            onClick={() => {
                              setConfirm(doc.id);
                              setError("");
                            }}
                          >
                            <TrashIcon className="h-3.5 w-3.5" />
                            Delete
                          </button>
                        )}
                      </div>

                      {confirm === doc.id && (
                        <div className="mt-2 rounded-xl border border-red-200 bg-red-50/80 p-3 text-left dark:border-red-900 dark:bg-red-950/60">
                          <p className="text-xs text-red-900 dark:text-red-200">
                            Delete <strong>{doc.filename}</strong> from this tender? It will be excluded from future TSCs.
                          </p>
                          <div className="mt-2 flex flex-wrap gap-2">
                            <button
                              type="button"
                              disabled={busy}
                              className="rounded-lg border border-slate-300 bg-white px-2.5 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-200"
                              onClick={() => setConfirm("")}
                            >
                              Keep file
                            </button>
                            <button
                              type="button"
                              disabled={busy}
                              className="rounded-lg bg-red-600 px-2.5 py-1 text-xs font-bold text-white hover:bg-red-700 disabled:opacity-50"
                              onClick={() => void remove(doc)}
                            >
                              {busy ? "Deleting…" : "Yes, delete"}
                            </button>
                          </div>
                        </div>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="rounded-xl border border-dashed border-slate-200 p-8 text-center text-xs text-slate-500 dark:border-slate-800 dark:text-slate-400">
          No extracted documents yet. Upload your tender files or ZIP archive above; verified files will appear here automatically.
        </div>
      )}
    </section>
  );
}
