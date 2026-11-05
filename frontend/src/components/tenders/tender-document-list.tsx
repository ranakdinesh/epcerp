"use client";
import { useState } from "react";
import { DocumentIcon, TrashIcon } from "@heroicons/react/24/outline";

export type TenderDocument = { id: string; filename: string; path: string; kind: string; byte_size: number; pages: number };
const action = "inline-flex min-h-11 items-center justify-center rounded-xl border px-3 text-sm font-semibold disabled:opacity-50";

export function TenderDocumentList({ draftID, version, documents, canWrite, changed }: {
  draftID: string; version: number; documents: TenderDocument[]; canWrite: boolean; changed: () => Promise<void>;
}) {
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  async function remove(document: TenderDocument) {
    if (busy) return;
    setBusy(true); setError(""); setMessage("");
    try {
      const response = await fetch(`/api/tender-fms/drafts/${draftID}/documents/${document.id}`, {
        method: "DELETE", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ expected_version: version }),
      });
      if (!response.ok) throw Error((await response.json()).detail || "Unable to delete document");
      await changed(); setConfirm(""); setMessage(`${document.filename} deleted from this tender.`);
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to delete document"); }
    finally { setBusy(false); }
  }
  return <section aria-label="Tender documents" className="space-y-3 rounded-xl border border-slate-200 p-4 dark:border-slate-700">
    <h2 className="font-bold">Tender documents ({documents.length})</h2>
    <p className="text-sm text-slate-600 dark:text-slate-300">Review the extracted files and delete anything you do not need. The remaining PDFs are available in Step 2.</p>
    {message ? <p role="status" className="text-sm">{message}</p> : null}
    {error ? <p role="alert" className="text-sm text-red-700 dark:text-red-300">{error}</p> : null}
    {documents.length ? <ul className="divide-y divide-slate-200 dark:divide-slate-700">{documents.map(document => <li key={document.id} className="space-y-3 py-3">
      <div className="grid grid-cols-[1.25rem_minmax(0,1fr)] items-start gap-x-3 gap-y-2 sm:grid-cols-[1.25rem_minmax(0,1fr)_auto]"><DocumentIcon className="mt-1 h-5 w-5 shrink-0 text-slate-500" />
        <div className="min-w-0 flex-1"><p className="break-all text-sm font-semibold">{document.filename}</p><p className="break-all text-xs text-slate-600 dark:text-slate-300">{document.path}</p><p className="mt-1 text-xs text-slate-600 dark:text-slate-300">{document.kind.toUpperCase()} · {(document.byte_size / 1048576).toFixed(2)} MiB{document.kind === "pdf" ? ` · ${document.pages} pages` : " · export to PDF for cloud TSC"}</p></div>
        <div className="col-start-2 flex flex-wrap gap-2 sm:col-start-3"><a className={`${action} border-slate-300 text-blue-700 dark:text-blue-300`} href={`/api/tender-fms/drafts/${draftID}/documents/${document.id}/original`} download={document.filename}>Download</a>{canWrite ? <button type="button" disabled={busy} className={`${action} gap-2 border-red-200 text-red-700 dark:border-red-800 dark:text-red-300`} aria-label={`Delete ${document.filename}`} onClick={() => { setConfirm(document.id); setError(""); }}><TrashIcon className="h-4 w-4" />Delete</button> : null}</div>
      </div>
      {confirm === document.id ? <div className="space-y-2 rounded-lg bg-slate-50 p-3 text-sm dark:bg-slate-800"><p>Delete {document.filename} from this tender? It will be excluded from new TSCs. Existing TSC evidence stays available.</p><div className="flex flex-wrap gap-2"><button type="button" disabled={busy} className={`${action} border-slate-300`} onClick={() => setConfirm("")}>Keep file</button><button type="button" disabled={busy} className={`${action} border-red-700 bg-red-700 text-white`} onClick={() => void remove(document)}>{busy ? "Deleting…" : "Delete from tender"}</button></div></div> : null}
    </li>)}</ul> : <p className="text-sm text-slate-600 dark:text-slate-300">No documents yet. Upload files or a ZIP above; verified documents will appear here.</p>}
  </section>;
}
