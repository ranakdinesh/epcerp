"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";

import { FormEvent, useEffect, useState } from "react";

import { O2teChecklist, type O2teStep } from "@/components/projects/o2te-checklist";

type DocumentRecord = {
  id: string;
  category: string;
  title: string;
  original_filename: string;
  content_type: string;
  byte_size: number;
  created_at: string;
};

type LifecycleEvent = {
  id: string;
  from_status: string;
  to_status: string;
  reason: string;
  created_at: string;
};

type InitiationRecord = {
  project: {
    id: string;
    project_number: string;
    name: string;
    lifecycle_status: "draft" | "active" | "on_hold" | "closed";
    loi_number: string;
    po_number: string;
  };
  documents: DocumentRecord[];
  events: LifecycleEvent[];
  o2te_steps: O2teStep[];
};

type ProjectInitiationPanelProps = {
  projectId: string;
  onClose: () => void;
  onChanged: () => void;
};

const DOCUMENT_CATEGORIES = [
  { value: "contract", label: "Contract" },
  { value: "loi", label: "Letter of Intent" },
  { value: "purchase_order", label: "Purchase Order" },
  { value: "work_order", label: "Work Order" },
  { value: "boq", label: "BOQ" },
  { value: "other", label: "Other" },
] as const;

function apiMessage(payload: unknown, fallback: string): string {
  if (typeof payload === "object" && payload !== null && "detail" in payload) {
    const { detail } = payload;
    if (typeof detail === "string") return detail;
  }
  if (typeof payload === "object" && payload !== null && "message" in payload) {
    const { message } = payload;
    if (typeof message === "string") return message;
  }
  return fallback;
}

function displayDate(value: string): string {
  return new Intl.DateTimeFormat("en-IN", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

async function requestInitiation(projectId: string): Promise<InitiationRecord> {
  const response = await fetch(`/api/projects/${projectId}/initiation`, { cache: "no-store" });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(apiMessage(payload, "Unable to load project initiation."));
  return payload as InitiationRecord;
}

function Icon({ name }: { name: "close" | "upload" | "download" | "arrow" }) {
  const paths = {
    close: <path d="m6 6 12 12M18 6 6 18" />,
    upload: <><path d="M12 16V3m0 0L7.5 7.5M12 3l4.5 4.5" /><path d="M5 13.5V20h14v-6.5" /></>,
    download: <><path d="M12 3v13m0 0 4.5-4.5M12 16l-4.5-4.5" /><path d="M5 20h14" /></>,
    arrow: <path d="M5 12h13m0 0-5-5m5 5-5 5" />,
  };
  return <svg aria-hidden="true" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="1.9">{paths[name]}</svg>;
}

export function ProjectInitiationPanel({ projectId, onClose, onChanged }: ProjectInitiationPanelProps) {
  const [record, setRecord] = useState<InitiationRecord | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isUploading, setIsUploading] = useState(false);
  const [isTransitioning, setIsTransitioning] = useState(false);
  const [category, setCategory] = useState("contract");
  const [title, setTitle] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [targetStatus, setTargetStatus] = useState("active");
  const [reason, setReason] = useState("");

  async function loadInitiation() {
    setIsLoading(true);
    setError("");
    try {
      setRecord(await requestInitiation(projectId));
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "Unable to load project initiation.");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    let current = true;
    void requestInitiation(projectId)
      .then((payload) => {
        if (current) setRecord(payload);
      })
      .catch((loadError: unknown) => {
        if (current) {
          setError(loadError instanceof Error ? loadError.message : "Unable to load project initiation.");
        }
      })
      .finally(() => {
        if (current) setIsLoading(false);
      });
    return () => {
      current = false;
    };
  }, [projectId]);

  async function uploadDocument(event: FormEvent<HTMLFormElement>) {
    const submittedForm = event.currentTarget instanceof HTMLFormElement ? event.currentTarget : undefined;
    event.preventDefault();
    if (!file) {
      setError("Choose a document before uploading.");
      return;
    }
    setIsUploading(true);
    setError("");
    setNotice("");
    const data = new FormData();
    data.set("file", file);
    try {
      const query = new URLSearchParams({ category, title: title.trim() });
      const response = await fetch(`/api/projects/${projectId}/documents?${query}`, {
        method: "POST",
        body: data,
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(apiMessage(payload, "Unable to upload the document."));
      setNotice("Contract evidence was added to the project record.");
      setTitle("");
      setFile(null);
      await loadInitiation();
      onChanged();
    } catch (uploadError) {
      if (displayServerErrors(submittedForm, uploadError)) return;
      setError(uploadError instanceof Error ? uploadError.message : "Unable to upload the document.");
    } finally {
      setIsUploading(false);
    }
  }

  async function transitionProject(event: FormEvent<HTMLFormElement>) {
    const submittedForm = event.currentTarget instanceof HTMLFormElement ? event.currentTarget : undefined;
    event.preventDefault();
    setIsTransitioning(true);
    setError("");
    setNotice("");
    try {
      const response = await fetch(`/api/projects/${projectId}/lifecycle`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ to_status: targetStatus, reason: reason.trim() }),
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(apiMessage(payload, "Unable to update the project lifecycle."));
      setNotice(`Project moved to ${targetStatus.replace("_", " ")}.`);
      setReason("");
      await loadInitiation();
      onChanged();
    } catch (transitionError) {
      if (displayServerErrors(submittedForm, transitionError)) return;
      setError(transitionError instanceof Error ? transitionError.message : "Unable to update the project lifecycle.");
    } finally {
      setIsTransitioning(false);
    }
  }

  const statusOptions = record?.project.lifecycle_status === "draft"
    ? ["active", "on_hold"]
    : record?.project.lifecycle_status === "active"
      ? ["on_hold", "closed"]
      : record?.project.lifecycle_status === "on_hold"
        ? ["active", "closed"]
        : [];

  return (
    <div className="fixed inset-0 z-50 flex items-end bg-slate-950/50 backdrop-blur-sm sm:items-center sm:justify-center sm:p-6">
      <section aria-labelledby="project-initiation-title" aria-modal="true" role="dialog" className="vtpl-modal max-h-[94dvh] w-full overflow-y-auto rounded-t-3xl bg-slate-50 shadow-2xl sm:max-w-5xl sm:rounded-3xl">
        <header className="sticky top-0 z-10 flex items-start justify-between border-b border-slate-200 bg-white px-5 py-5 sm:px-7">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.16em] text-blue-700">Project control</p>
            <h3 id="project-initiation-title" className="mt-1 text-xl font-extrabold text-slate-900">Project initiation</h3>
            {record ? <p className="mt-1 text-sm text-slate-500"><span className="font-mono font-bold text-blue-700">{record.project.project_number}</span> · {record.project.name}</p> : null}
          </div>
          <button aria-label="Close project initiation" className="grid min-h-11 min-w-11 place-items-center rounded-xl text-slate-500 transition hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500" onClick={onClose} type="button"><Icon name="close" /></button>
        </header>

        <div className="space-y-5 p-5 sm:p-7">
          {error ? <p role="alert" className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm font-semibold text-red-800">{error}</p> : null}
          {notice ? <p role="status" className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-semibold text-emerald-800">{notice}</p> : null}
          {isLoading ? <p className="rounded-2xl border border-slate-200 bg-white p-8 text-center text-sm font-medium text-slate-500">Loading initiation controls...</p> : null}

          {record ? <>
            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-2xl border border-slate-200 bg-white p-4"><p className="text-[11px] font-bold uppercase tracking-[0.12em] text-slate-500">Lifecycle</p><p className="mt-2 text-lg font-extrabold capitalize text-slate-900">{record.project.lifecycle_status.replace("_", " ")}</p></div>
              <div className="rounded-2xl border border-slate-200 bg-white p-4"><p className="text-[11px] font-bold uppercase tracking-[0.12em] text-slate-500">LOI reference</p><p className="mt-2 truncate text-sm font-bold text-slate-800">{record.project.loi_number || "Not recorded"}</p></div>
              <div className="rounded-2xl border border-slate-200 bg-white p-4"><p className="text-[11px] font-bold uppercase tracking-[0.12em] text-slate-500">PO reference</p><p className="mt-2 truncate text-sm font-bold text-slate-800">{record.project.po_number || "Not recorded"}</p></div>
            </div>

            <O2teChecklist
              onUpdated={loadInitiation}
              projectId={projectId}
              steps={record.o2te_steps}
            />

            <div className="grid gap-5 lg:grid-cols-[1.2fr_0.8fr]">
              <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white">
                <div className="border-b border-slate-100 px-5 py-4"><h4 className="font-extrabold text-slate-900">Contract evidence</h4><p className="mt-1 text-sm text-slate-500">Upload the source record before activating delivery work.</p></div>
                <div className="divide-y divide-slate-100">
                  {record.documents.length ? record.documents.map((document) => <div className="flex items-center justify-between gap-3 px-5 py-4" key={document.id}><div className="min-w-0"><p className="truncate text-sm font-bold text-slate-800">{document.title}</p><p className="mt-1 truncate text-xs text-slate-500">{document.category.replaceAll("_", " ")} · {document.original_filename} · {Math.ceil(document.byte_size / 1024)} KB</p></div><a aria-label={`Download ${document.title}`} className="grid min-h-11 min-w-11 shrink-0 place-items-center rounded-xl border border-slate-200 text-slate-600 transition hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500" href={`/api/projects/${projectId}/documents/${document.id}/download`}><Icon name="download" /></a></div>) : <p className="px-5 py-10 text-center text-sm text-slate-500">No contract evidence has been uploaded.</p>}
                </div>
                <ValidatedForm className="border-t border-slate-100 bg-slate-50 p-5" onSubmit={(event) => void uploadDocument(event)}>
                  <div className="grid gap-3 sm:grid-cols-2"><ValidatedLabel className="text-sm font-bold text-slate-700">Document type<ValidatedSelect name="category" className="field" onChange={(event) => setCategory(event.target.value)} value={category}>{DOCUMENT_CATEGORIES.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</ValidatedSelect></ValidatedLabel><ValidatedLabel className="text-sm font-bold text-slate-700">Document title<ValidatedInput name="title" className="field" maxLength={255} onChange={(event) => setTitle(event.target.value)} placeholder="Defaults to filename" value={title} /></ValidatedLabel></div>
                  <ValidatedLabel className="mt-3 block text-sm font-bold text-slate-700">File<ValidatedInput accept=".pdf,.png,.jpg,.jpeg,.docx" className="mt-1 block w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 file:mr-3 file:rounded-lg file:border-0 file:bg-blue-50 file:px-3 file:py-2 file:text-sm file:font-bold file:text-blue-700" onChange={(event) => setFile(event.target.files?.[0] ?? null)} type="file" /></ValidatedLabel>
                  <p className="mt-2 text-xs text-slate-500">PDF, PNG, JPEG, or DOCX. The selected file is validated by the server.</p>
                  <button className="mt-4 inline-flex min-h-11 items-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white shadow-sm transition hover:bg-blue-800 disabled:cursor-not-allowed disabled:opacity-60 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2" disabled={isUploading} type="submit"><Icon name="upload" />{isUploading ? "Uploading..." : "Add evidence"}</button>
                </ValidatedForm>
              </div>

              <div className="space-y-5">
                <ValidatedForm className="rounded-2xl border border-slate-200 bg-white p-5" onSubmit={(event) => void transitionProject(event)}>
                  <h4 className="font-extrabold text-slate-900">Lifecycle decision</h4>
                  <p className="mt-1 text-sm leading-6 text-slate-500">Activation requires a recorded LOI or PO and uploaded contract evidence.</p>
                  {statusOptions.length ? <><ValidatedLabel className="mt-4 block text-sm font-bold text-slate-700">Move project to<ValidatedSelect name="target_status" className="field" onChange={(event) => setTargetStatus(event.target.value)} value={targetStatus}>{statusOptions.map((status) => <option key={status} value={status}>{status.replace("_", " ")}</option>)}</ValidatedSelect></ValidatedLabel><ValidatedLabel className="mt-3 block text-sm font-bold text-slate-700">Reason <span className="font-medium text-slate-400">required for hold or closure</span><ValidatedTextarea name="reason" className="field" maxLength={2000} onChange={(event) => setReason(event.target.value)} rows={3} value={reason} /></ValidatedLabel><button className="mt-4 inline-flex min-h-11 items-center gap-2 rounded-xl border border-blue-200 bg-blue-50 px-4 text-sm font-bold text-blue-800 transition hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-60 focus:outline-none focus:ring-2 focus:ring-blue-500" disabled={isTransitioning} type="submit">{isTransitioning ? "Updating..." : <><span>Record decision</span><Icon name="arrow" /></>}</button></> : <p className="mt-4 rounded-xl bg-slate-50 px-3 py-3 text-sm font-medium text-slate-600">This project is closed; no further lifecycle transition is available.</p>}
                </ValidatedForm>

                <section className="rounded-2xl border border-slate-200 bg-white p-5"><h4 className="font-extrabold text-slate-900">Decision history</h4><div className="mt-4 space-y-3">{record.events.length ? record.events.map((event) => <div className="border-l-2 border-blue-200 pl-3" key={event.id}><p className="text-sm font-bold capitalize text-slate-800">{event.from_status.replace("_", " ")} <span className="text-slate-400">to</span> {event.to_status.replace("_", " ")}</p><p className="mt-1 text-xs text-slate-500">{displayDate(event.created_at)}{event.reason ? ` · ${event.reason}` : ""}</p></div>) : <p className="text-sm text-slate-500">No lifecycle decision has been recorded.</p>}</div></section>
              </div>
            </div>
          </> : null}
        </div>
      </section>
    </div>
  );
}
