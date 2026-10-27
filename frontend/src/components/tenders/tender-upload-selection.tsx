"use client";

import { useRef, useState } from "react";
import { ArrowUpTrayIcon, DocumentIcon, PlusIcon, XMarkIcon } from "@heroicons/react/24/outline";
import { ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";
import { packageFileError } from "./tender-capture-model";

export function TenderUploadSelection({ files, changed, error, setError, disabled, locked }: {
  files: File[]; changed: (files: File[]) => void; error: string; setError: (error: string) => void; disabled: boolean; locked: boolean;
}) {
  const picker = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const unavailable = disabled || locked;
  function add(incoming: File[]) {
    if (unavailable || !incoming.length) return;
    const next = [...files, ...incoming];
    const issue = packageFileError(next);
    setError(issue);
    if (!issue) changed(next);
  }
  return <div className="space-y-3" aria-label="Tender document selection">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <ValidatedLabel required htmlFor="reading-files" className="font-semibold">Documents or ZIP</ValidatedLabel>
      <button type="button" disabled={unavailable} onClick={() => picker.current?.click()} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white hover:bg-blue-800 disabled:opacity-50"><PlusIcon className="h-5 w-5" />Add documents</button>
    </div>
    <button type="button" disabled={unavailable} aria-label="Choose documents or drop files" aria-describedby="reading-file-help" onClick={() => picker.current?.click()}
      onDragOver={event => { event.preventDefault(); if (!unavailable) setDragging(true); }}
      onDragLeave={() => setDragging(false)}
      onDrop={event => { event.preventDefault(); setDragging(false); add(Array.from(event.dataTransfer.files)); }}
      className={`flex min-h-36 w-full flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed p-6 text-center transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600 disabled:cursor-not-allowed disabled:opacity-60 ${dragging ? "border-blue-600 bg-blue-100 dark:bg-blue-950" : "border-slate-400 bg-slate-50 hover:border-blue-600 hover:bg-blue-50 dark:border-slate-500 dark:bg-slate-800 dark:hover:border-blue-400 dark:hover:bg-slate-800/70"}`}>
      <ArrowUpTrayIcon className="h-8 w-8 text-blue-700 dark:text-blue-300" />
      <span className="font-semibold">Click to choose documents or drag and drop here</span>
      <span className="text-xs text-slate-600 dark:text-slate-300">Add files together or one by one. Earlier selections stay in the list.</span>
    </button>
    <ValidatedInput ref={picker} id="reading-files" name="files" type="file" accept=".pdf,.docx,.xlsx,.zip" multiple tabIndex={-1} className="sr-only" disabled={unavailable} error={error} aria-describedby="reading-file-help" onChange={event => { add(Array.from(event.currentTarget.files || [])); event.currentTarget.value = ""; }} />
    <p id="reading-file-help" className="text-xs text-slate-600 dark:text-slate-300">PDF, DOCX, XLSX or ZIP. Up to 32 MiB per document, 64 MiB per ZIP and 250 MiB per batch.</p>
    {locked ? <p className="text-sm text-amber-800 dark:text-amber-300">This batch has already been prepared. Finish its upload before adding more documents in a new batch.</p> : null}
    <p role="status" className="text-sm">{files.length ? `${files.length} document${files.length === 1 ? "" : "s"} selected · ${(files.reduce((total, file) => total + file.size, 0) / 1024 / 1024).toFixed(2)} MiB` : "No documents selected. Use Add documents to start."}</p>
    {files.length ? <ul aria-label="Selected documents" className="divide-y divide-slate-200 rounded-xl border border-slate-200 dark:divide-slate-700 dark:border-slate-600">{files.map(file => <li key={file.name.toLowerCase()} className="flex items-center gap-3 p-3"><DocumentIcon className="h-5 w-5 shrink-0 text-slate-500" /><div className="min-w-0 flex-1"><p className="break-all text-sm font-semibold">{file.name}</p><p className="text-xs text-slate-500 dark:text-slate-300">{(file.size / 1024).toFixed(1)} KiB · Ready to upload</p></div><button type="button" disabled={unavailable} aria-label={`Remove ${file.name}`} title={`Remove ${file.name}`} className="inline-flex min-h-11 min-w-11 shrink-0 items-center justify-center rounded-xl text-slate-600 hover:bg-slate-100 disabled:opacity-50 dark:text-slate-200 dark:hover:bg-slate-700" onClick={() => { changed(files.filter(candidate => candidate !== file)); setError(""); }}><XMarkIcon className="h-5 w-5" /></button></li>)}</ul> : null}
  </div>;
}
