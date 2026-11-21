"use client";

import { useState } from "react";
import {
  DocumentTextIcon,
  TrashIcon,
  EyeIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
} from "@heroicons/react/24/outline";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ValidatedForm } from "@/components/ui/validated-controls";

export type TenderDocument = {
  id: string;
  filename: string;
  path: string;
  kind: string;
  byte_size: number;
  pages: number;
};

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
  const [confirmDoc, setConfirmDoc] = useState<TenderDocument | null>(null);
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
      setConfirmDoc(null);
      setMessage(`${document.filename} has been permanently deleted.`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to delete document");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section
      aria-label="Attached tender documents"
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

      <DataGrid
        stateKey="tender_attached_documents"
        label="Tender documents"
        rows={documents}
        searchable
        paginate
        columns={[
          {
            key: "filename",
            label: "Document Name",
            value: (doc) => doc.filename,
            sortable: true,
            render: (doc) => (
              <div className="flex items-center gap-2.5 py-1">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                  <DocumentTextIcon className="h-4 w-4" />
                </div>
                <div className="min-w-0">
                  <p className="truncate font-semibold text-slate-900 dark:text-white">
                    {doc.filename}
                  </p>
                  {doc.path && doc.path !== doc.filename && (
                    <p className="truncate text-[11px] text-slate-500 dark:text-slate-400">
                      {doc.path}
                    </p>
                  )}
                </div>
              </div>
            ),
          },
          {
            key: "kind",
            label: "Format",
            value: (doc) => doc.kind.toUpperCase(),
            sortable: true,
            render: (doc) => (
              <span className="rounded-md bg-slate-100 px-2 py-0.5 text-[11px] font-bold text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                {doc.kind.toUpperCase()}
              </span>
            ),
          },
          {
            key: "size",
            label: "Size",
            value: (doc) => doc.byte_size,
            sortable: true,
            render: (doc) => (
              <span className="text-xs text-slate-600 dark:text-slate-400">
                {doc.byte_size >= 1048576
                  ? `${(doc.byte_size / 1048576).toFixed(2)} MiB`
                  : `${(doc.byte_size / 1024).toFixed(1)} KiB`}
              </span>
            ),
          },
          {
            key: "status",
            label: "Status",
            value: (doc) => (doc.kind === "pdf" ? `Ready · ${doc.pages || 0} pgs` : "Reference"),
            sortable: false,
            render: (doc) =>
              doc.kind === "pdf" ? (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-2.5 py-0.5 text-[11px] font-semibold text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300">
                  <span className="h-1.5 w-1.5 rounded-full bg-emerald-600" />
                  Ready · {doc.pages || 0} pgs
                </span>
              ) : (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-amber-50 px-2.5 py-0.5 text-[11px] font-semibold text-amber-700 dark:bg-amber-950/60 dark:text-amber-300">
                  Reference
                </span>
              ),
          },
        ]}
        actions={(doc) => (
          <div className="flex items-center justify-end gap-1">
            <a
              href={`/api/tender-fms/drafts/${draftID}/documents/${doc.id}/original`}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-blue-700 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-950"
              title={`View ${doc.filename}`}
              aria-label={`View ${doc.filename}`}
            >
              <EyeIcon className="h-5 w-5" />
            </a>
            {canWrite && (
              <button
                type="button"
                disabled={busy}
                className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/60 disabled:opacity-50"
                title={`Permanently delete ${doc.filename}`}
                aria-label={`Delete ${doc.filename}`}
                onClick={() => setConfirmDoc(doc)}
              >
                <TrashIcon className="h-5 w-5" />
              </button>
            )}
          </div>
        )}
      />

      {confirmDoc && (
        <RecordDrawer
          title="Delete Document"
          onClose={() => setConfirmDoc(null)}
        >
          <ValidatedForm
            className="space-y-4"
            onSubmit={async (e) => {
              e.preventDefault();
              await remove(confirmDoc);
            }}
          >
            <div className="rounded-xl border border-red-200 bg-red-50/60 p-4 dark:border-red-900/50 dark:bg-red-950/20">
              <p className="text-sm font-semibold text-slate-900 dark:text-white">
                Permanently delete document?
              </p>
              <p className="mt-1 text-xs text-slate-600 dark:text-slate-400">
                <strong>{confirmDoc.filename}</strong> will be permanently deleted from this tender draft and will no longer be available in Step 2 for TSC evaluation.
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
                className="inline-flex min-h-11 items-center justify-center rounded-xl border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                disabled={busy}
                onClick={() => setConfirmDoc(null)}
              >
                Cancel
              </button>
              <button
                type="submit"
                className="inline-flex min-h-11 items-center justify-center rounded-xl bg-red-600 px-4 text-sm font-bold text-white hover:bg-red-700 disabled:opacity-50"
                disabled={busy}
              >
                {busy ? "Deleting…" : "Permanently Delete"}
              </button>
            </div>
          </ValidatedForm>
        </RecordDrawer>
      )}
    </section>
  );
}
