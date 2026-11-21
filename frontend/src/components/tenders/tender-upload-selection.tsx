"use client";

import { useRef, useState } from "react";
import { ArrowUpTrayIcon, DocumentIcon, PlusIcon, XMarkIcon } from "@heroicons/react/24/outline";
import { ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";
import { packageFileError } from "./tender-capture-model";

export function TenderUploadSelection({
  files,
  changed,
  error,
  setError,
  disabled,
  locked,
}: {
  files: File[];
  changed: (files: File[]) => void;
  error: string;
  setError: (error: string) => void;
  disabled: boolean;
  locked: boolean;
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

  const totalBytes = files.reduce((total, file) => total + file.size, 0);

  return (
    <div className="space-y-4" aria-label="Tender document selection">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <ValidatedLabel required htmlFor="reading-files" className="text-sm font-bold text-slate-800 dark:text-slate-200">
            Source Documents or ZIP
          </ValidatedLabel>
          <p id="reading-file-help" className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
            PDF, DOCX, XLSX or ZIP · Max 32 MiB per document, 64 MiB per ZIP, 250 MiB per batch
          </p>
        </div>
        <button
          type="button"
          disabled={unavailable}
          onClick={() => picker.current?.click()}
          className="inline-flex min-h-10 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-xs font-bold text-white shadow-sm transition hover:bg-blue-800 disabled:opacity-50"
        >
          <PlusIcon className="h-4 w-4" />
          Choose files
        </button>
      </div>

      <button
        type="button"
        disabled={unavailable}
        aria-label="Choose documents or drop files"
        aria-describedby="reading-file-help"
        onClick={() => picker.current?.click()}
        onDragOver={(event) => {
          event.preventDefault();
          if (!unavailable) setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(event) => {
          event.preventDefault();
          setDragging(false);
          add(Array.from(event.dataTransfer.files));
        }}
        className={`group flex min-h-32 w-full flex-col items-center justify-center gap-2 rounded-2xl border-2 border-dashed p-6 text-center transition-all focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600 disabled:cursor-not-allowed disabled:opacity-60 ${
          dragging
            ? "border-blue-600 bg-blue-50/80 dark:border-blue-500 dark:bg-blue-950/40"
            : "border-slate-300 bg-slate-50/60 hover:border-blue-500 hover:bg-blue-50/40 dark:border-slate-700 dark:bg-slate-850 dark:hover:border-blue-400 dark:hover:bg-slate-800/80"
        }`}
      >
        <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-white shadow-sm transition group-hover:scale-105 group-hover:bg-blue-600 group-hover:text-white dark:bg-slate-800 dark:text-slate-300">
          <ArrowUpTrayIcon className="h-5 w-5 text-blue-700 transition group-hover:text-white dark:text-blue-400" />
        </div>
        <div>
          <span className="text-sm font-semibold text-slate-800 dark:text-slate-200">
            Click to browse documents or drag and drop here
          </span>
          <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
            Add files one by one or in batches. Earlier selections stay in the list.
          </p>
        </div>
      </button>

      <ValidatedInput
        ref={picker}
        id="reading-files"
        name="files"
        type="file"
        accept=".pdf,.docx,.xlsx,.zip"
        multiple
        tabIndex={-1}
        className="sr-only"
        disabled={unavailable}
        error={error}
        aria-describedby="reading-file-help"
        onChange={(event) => {
          add(Array.from(event.currentTarget.files || []));
          event.currentTarget.value = "";
        }}
      />

      {locked && (
        <p className="text-xs text-slate-600 dark:text-slate-400">
          These files are reserved for this upload batch. If the upload failed, retry with the same files.
        </p>
      )}

      {files.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center justify-between text-xs font-semibold text-slate-600 dark:text-slate-300">
            <span>Selected for upload ({files.length})</span>
            <span>{(totalBytes / 1024 / 1024).toFixed(2)} MiB total</span>
          </div>

          <ul
            aria-label="Selected documents"
            className="divide-y divide-slate-100 rounded-xl border border-slate-200 bg-white dark:divide-slate-800 dark:border-slate-800 dark:bg-slate-900"
          >
            {files.map((file) => {
              const ext = file.name.split(".").pop()?.toUpperCase() || "DOC";
              return (
                <li
                  key={file.name.toLowerCase()}
                  className="flex items-center justify-between gap-3 px-4 py-2.5 transition hover:bg-slate-50/80 dark:hover:bg-slate-800/50"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-[10px] font-bold text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                      {ext}
                    </span>
                    <div className="min-w-0">
                      <p className="truncate text-xs font-semibold text-slate-900 dark:text-white">
                        {file.name}
                      </p>
                      <p className="text-[11px] text-slate-500 dark:text-slate-400">
                        {(file.size / 1024).toFixed(1)} KiB · Ready to upload
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    disabled={unavailable}
                    aria-label={`Remove ${file.name}`}
                    title={`Remove ${file.name}`}
                    className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-lg text-slate-400 transition hover:bg-red-50 hover:text-red-700 dark:hover:bg-red-950 dark:hover:text-red-300"
                    onClick={() => {
                      changed(files.filter((candidate) => candidate !== file));
                      setError("");
                    }}
                  >
                    <XMarkIcon className="h-4 w-4" />
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </div>
  );
}
