"use client";

import {
  DocumentTextIcon,
  CheckCircleIcon,
  ArrowRightIcon,
  MagnifyingGlassIcon,
  ArrowDownTrayIcon,
  ClockIcon,
  SparklesIcon,
  PencilSquareIcon,
  LightBulbIcon,
  ExclamationTriangleIcon,
} from "@heroicons/react/24/outline";
import { useCallback, useEffect, useRef, useState, useMemo } from "react";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { createRecordId } from "@/lib/record-id";
import { TSCWorkspace } from "./tsc-workspace";
import { ValidatedInput, ValidatedSelect, ValidatedTextarea, ValidatedLabel } from "@/components/ui/validated-controls";
import type { Answer, Question, Source, Snapshot } from "@/server/tenders/tsc/types";

const button = "ui-action";
const primary = "ui-action ui-action--primary";
const failure = (e: unknown) => (e instanceof Error ? e.message : "Unable to complete TSC request");

type Cost = {
  estimated_micro_inr: number;
  unknown_requests: number;
  unconfirmed_reserved_paise: number;
  legacy_reserved_paise: number;
  limit_paise: number;
  requests: {
    id: string;
    job_id: string;
    response_id: string | null;
    http_status: number | null;
    model: string;
    state: string;
    estimated_micro_inr: number | null;
    usage?: { input_tokens?: number; output_tokens?: number };
    pricing: { usd_inr: number; fx_date: string; tier: string };
  }[];
};

type Options = {
  engine?: string;
  upload_batches?: { id: string; state: string; error_code: string | null; blocked_path: string | null }[];
  formats: { id: string; version: number; name: string; question_count: number }[];
  sources: (Source & { reference_document: boolean })[];
  providers: {
    mode: string;
    available: boolean;
    model: string;
    reservation_paise: number;
    quality_model?: string | null;
    quality_reservation_paise?: number | null;
    quality_minimum_reservation_paise?: number | null;
    input_method?: string;
    name?: string;
  }[];
  cost: Cost;
};

type Summary = {
  preparation_method?: string;
  id: string;
  state: string;
  mode: string;
  engine?: string;
  error_code?: string;
  created_at: string;
};

type Job = Summary & {
  version: number;
  draft_version: number;
  stale: boolean;
  error_code: string;
  snapshot: Snapshot;
  answers: Answer[];
  cost: Cost;
  tasks: { id: string; state: string; question_id: string | null; error_code: string }[];
};

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/tender-fms/${path}`, { cache: "no-store", ...init });
  const data = await response.json().catch(() => ({ detail: "Unreadable TSC response" }));
  if (!response.ok) throw Error(data.detail || "Unable to complete TSC request");
  return data;
}

const write = (value: unknown, key?: string, method = "POST"): RequestInit => ({
  method,
  headers: { "Content-Type": "application/json", ...(key ? { "Idempotency-Key": key } : {}) },
  body: JSON.stringify(value),
});

function formatSectionName(raw: string): string {
  if (!raw) return "General Questions";
  return raw
    .replaceAll("_", " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

export function CloudTSCWorkspace(props: {
  draftID: string;
  sourceVersion: number;
  readyForGeneration?: boolean;
  onDone?: () => void;
  onDirty?: (value: boolean) => void;
  onProceedToStep3?: () => void;
}) {
  const [options, setOptions] = useState<Options | null>(null);
  const [error, setError] = useState("");

  const refresh = useCallback(
    async (signal?: AbortSignal) => {
      try {
        setOptions(await api<Options>(`drafts/${props.draftID}/tsc/options`, { signal }));
        setError("");
      } catch (e) {
        if (!signal?.aborted) setError(failure(e));
      }
    },
    [props.draftID]
  );

  useEffect(() => {
    const c = new AbortController();
    const timer = setTimeout(() => void refresh(c.signal), 0);
    const poll = setInterval(() => void refresh(c.signal), 12000);
    return () => {
      clearTimeout(timer);
      clearInterval(poll);
      c.abort();
    };
  }, [refresh, props.sourceVersion]);

  if (error && !options) {
    return (
      <p role="alert">
        {error} <button className={button} onClick={() => void refresh()}>Retry</button>
      </p>
    );
  }
  if (!options) return <p role="status">Loading TSC workspace…</p>;
  if (options.engine !== "nextjs") return <TSCWorkspace {...props} />;

  return (
    <>
      {error && (
        <p role="alert" className="tsc-warning">
          {error} Your current checklist work is retained.{" "}
          <button className={button} onClick={() => void refresh()}>
            Retry
          </button>
        </p>
      )}
      <NativeTSC
        key={props.draftID}
        {...props}
        options={options}
        refreshOptions={refresh}
        onDone={props.onDone}
        onDirty={props.onDirty}
        onProceedToStep3={props.onProceedToStep3}
      />
    </>
  );
}

function NativeTSC({
  draftID,
  sourceVersion,
  options,
  refreshOptions,
  onDirty,
  onProceedToStep3,
}: {
  draftID: string;
  sourceVersion: number;
  options: Options;
  refreshOptions: () => Promise<void>;
  onDone?: () => void;
  onDirty?: (value: boolean) => void;
  onProceedToStep3?: () => void;
}) {
  const user = useAuthenticatedUser();
  const has = (p: string) => user.permissions.includes("system:admin") || user.permissions.includes(p);
  const canWrite = has("tenders.tsc.write") && has("tenders.register.write") && has("tenders.processing.write");
  const canCloud = canWrite && has("tenders.tsc.originals.write");
  const base = `drafts/${draftID}/tsc`;

  const [jobs, setJobs] = useState<Summary[]>([]);
  const [selected, setSelected] = useState("");
  const [job, setJob] = useState<Job | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  // Grid editing states
  const [workingDrafts, setWorkingDrafts] = useState<Record<string, { answer: string; status: Answer["status"] }>>({});
  const [rowStatus, setRowStatus] = useState<Record<string, "idle" | "saving" | "saved" | "error">>({});
  const [activeId, setActiveId] = useState<string | null>(null);

  // Filters
  const [searchQuery, setSearchQuery] = useState("");
  const [sectionFilter, setSectionFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState<"all" | "unfilled" | "answered">("all");

  const operation = useRef<{ fingerprint: string; key: string } | null>(null);
  const initialized = useRef(false);
  const latestVersionRef = useRef<number>(1);
  const saveQueue = useRef<Promise<void>>(Promise.resolve());
  const unsavedRef = useRef(false);

  const markDirty = (value: boolean) => {
    unsavedRef.current = value;
    onDirty?.(value);
  };

  useEffect(() => () => onDirty?.(false), [onDirty]);
  useEffect(() => {
    void refreshOptions();
  }, [job?.version, refreshOptions]);

  const refresh = useCallback(
    async (signal?: AbortSignal) => {
      try {
        const data = await api<{ items: Summary[] }>(base, { signal });
        setJobs(data.items);
        const targetId = selected || data.items[0]?.id;
        if (!initialized.current && data.items[0]?.id) {
          initialized.current = true;
          setSelected(data.items[0].id);
        }
        if (targetId) {
          const summary = data.items.find((j) => j.id === targetId);
          if (summary?.engine !== "go") {
            const loadedJob = await api<Job>(`${base}/${targetId}`, { signal });
            setJob(loadedJob);
            latestVersionRef.current = loadedJob.version;
          } else {
            setJob(null);
          }
        }
        setError("");
      } catch (e) {
        if (!signal?.aborted) setError(failure(e));
      }
    },
    [base, selected]
  );

  useEffect(() => {
    const c = new AbortController();
    let live = true;
    let timer: ReturnType<typeof setTimeout>;
    async function poll() {
      await refresh(c.signal);
      if (live) timer = setTimeout(() => void poll(), 3000);
    }
    void poll();
    return () => {
      live = false;
      c.abort();
      clearTimeout(timer);
    };
  }, [refresh]);

  const [selectedFormatId, setSelectedFormatId] = useState<string>(options.formats[0]?.id || "");
  const [selectedProvider, setSelectedProvider] = useState<string>(
    options.providers.find((p) => p.available)?.mode || "gemini"
  );

  const selectedFormat = options.formats.find((f) => f.id === selectedFormatId) || options.formats[0];
  const pdfSources = useMemo(
    () => options.sources.filter((s) => !s.reference_document && s.kind === "pdf"),
    [options.sources]
  );
  const availableProviders = useMemo(
    () => options.providers.filter((p) => p.available),
    [options.providers]
  );
  const activeProvider =
    options.providers.find((p) => p.mode === selectedProvider) ||
    availableProviders[0] ||
    options.providers[0];

  // Initialize checklist if no job exists at all, or trigger a new generation
  async function generate(mode: string = "manual") {
    if (!selectedFormat || busy) return;
    setBusy(true);
    setError("");
    try {
      const request = {
        expected_version: sourceVersion,
        format_id: selectedFormat.id,
        format_version: selectedFormat.version,
        mode,
        source_ids: mode === "manual" ? [] : pdfSources.map((s) => s.id),
        auto_escalate: false,
        stronger_model: false,
        recover_overload: false,
        original_documents_confirmed: mode !== "manual",
      };
      const fingerprint = JSON.stringify(request);
      if (operation.current?.fingerprint !== fingerprint) {
        operation.current = { fingerprint, key: createRecordId() };
      }
      const next = await api<Job>(base, write(request, operation.current.key));
      operation.current = null;
      initialized.current = true;
      setSelected(next.id);
      setJob(next);
      latestVersionRef.current = next.version;
      setJobs((prev) => [next, ...prev]);
      await refreshOptions();
    } catch (e) {
      setError(failure(e));
    } finally {
      setBusy(false);
    }
  }

  // Populate working drafts directly whenever job is loaded/refreshed
  useEffect(() => {
    if (!job) return;
    setWorkingDrafts((prev) => {
      const next: Record<string, { answer: string; status: Answer["status"] }> = {};
      for (const a of job.answers) {
        // If user has actively unsaved changes for this specific question, keep them
        if (unsavedRef.current && prev[a.question_id] && prev[a.question_id].answer !== (a.answer || "")) {
          next[a.question_id] = prev[a.question_id];
        } else {
          next[a.question_id] = { answer: a.answer || "", status: a.status || "needs_review" };
        }
      }
      return next;
    });
  }, [job?.id, job?.version]);

  // Unique sections list
  const sections = useMemo(() => {
    if (!job?.snapshot.questions) return [];
    return Array.from(new Set(job.snapshot.questions.map((q) => q.section)));
  }, [job]);

  // Filtered questions
  const filteredQuestions = useMemo(() => {
    if (!job?.snapshot.questions) return [];
    return job.snapshot.questions.filter((q) => {
      const current = workingDrafts[q.id]?.answer ?? "";
      const matchesSearch =
        q.question.toLowerCase().includes(searchQuery.toLowerCase()) ||
        q.section.toLowerCase().includes(searchQuery.toLowerCase()) ||
        (q.printed_number && q.printed_number.includes(searchQuery));
      const matchesSection = sectionFilter === "all" || q.section === sectionFilter;
      const matchesStatus =
        statusFilter === "all" ||
        (statusFilter === "unfilled" && !current.trim()) ||
        (statusFilter === "answered" && Boolean(current.trim()));

      return matchesSearch && matchesSection && matchesStatus;
    });
  }, [job, searchQuery, sectionFilter, statusFilter, workingDrafts]);

  // Progress counts
  const totalQuestions = job?.snapshot.questions.length ?? 0;
  const answeredCount = useMemo(() => {
    if (!job?.snapshot.questions) return 0;
    return job.snapshot.questions.filter((q) => (workingDrafts[q.id]?.answer ?? "").trim().length > 0).length;
  }, [job, workingDrafts]);

  const progressPercent = totalQuestions > 0 ? Math.round((answeredCount / totalQuestions) * 100) : 0;

  // Handle local text change
  function handleTextChange(qId: string, text: string) {
    setWorkingDrafts((prev) => ({
      ...prev,
      [qId]: {
        answer: text,
        status: text.trim() ? "answered" : "needs_review",
      },
    }));
    markDirty(true);
  }

  // Handle local status change
  function handleStatusChange(qId: string, status: Answer["status"]) {
    setWorkingDrafts((prev) => ({
      ...prev,
      [qId]: {
        answer: prev[qId]?.answer ?? "",
        status,
      },
    }));
    markDirty(true);
    saveQuestion(qId, status);
  }

  // Auto-save on blur or row change
  function saveQuestion(qId: string, explicitStatus?: Answer["status"]) {
    if (!canWrite || !job) return;
    const current = workingDrafts[qId];
    if (!current) return;

    const original = job.answers.find((a) => a.question_id === qId);
    const targetStatus = explicitStatus ?? current.status;

    if (original && original.answer === current.answer && original.status === targetStatus) {
      return;
    }

    setRowStatus((prev) => ({ ...prev, [qId]: "saving" }));

    saveQueue.current = saveQueue.current.then(async () => {
      try {
        const payload = {
          expected_version: latestVersionRef.current,
          reason: "Working answer recorded",
          answer: {
            question_id: qId,
            answer: current.answer,
            status: targetStatus,
            citations: original?.citations ?? [],
            accepted: false,
            origin: "manual",
          },
        };

        const next = await api<Job>(
          `${base}/${job.id}/answers/${encodeURIComponent(qId)}`,
          write(payload, undefined, "PATCH")
        );

        latestVersionRef.current = next.version;
        setJob(next);
        setRowStatus((prev) => ({ ...prev, [qId]: "saved" }));
        markDirty(false);
      } catch (err) {
        console.error("Auto-save failed for", qId, err);
        try {
          const fresh = await api<Job>(`${base}/${job.id}`);
          latestVersionRef.current = fresh.version;
          setJob(fresh);
        } catch {
          // ignore
        }
        setRowStatus((prev) => ({ ...prev, [qId]: "error" }));
      }
    });
  }

  async function handleProceed() {
    await saveQueue.current;
    if (onProceedToStep3) {
      onProceedToStep3();
    }
  }

  return (
    <section aria-label="Tender Selection Checklist Grid" className="w-full space-y-6">
      {error && (
        <div role="alert" className="rounded-xl border border-red-200 bg-red-50 p-4 text-xs font-semibold text-red-800 dark:border-red-900 dark:bg-red-950/60 dark:text-red-300">
          {error}{" "}
          <button className={button + " ml-2 text-xs"} onClick={() => void refresh()}>
            Refresh Status
          </button>
        </div>
      )}

      {!jobs.length && busy ? (
        <div className="flex flex-col items-center justify-center rounded-2xl border border-dashed border-blue-300 bg-blue-50/40 p-12 text-center dark:border-blue-900 dark:bg-blue-950/20">
          <SparklesIcon className="h-10 w-10 animate-spin text-blue-600" />
          <p className="mt-3 text-sm font-bold text-slate-800 dark:text-slate-200">
            Initializing Technical Scrutiny Questionnaire…
          </p>
          <span className="text-xs text-slate-500">Setting up checklist criteria and preparing scrutiny grid.</span>
        </div>
      ) : !jobs.length ? (
        <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div>
              <div className="flex items-center gap-2">
                <span className="text-xs font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
                  Step 2 · Technical Scrutiny Checklist (TSC / TMC)
                </span>
              </div>
              <h2 className="mt-1 text-xl font-bold text-slate-900 dark:text-white">
                Prepare Technical Scrutiny Checklist
              </h2>
              <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                Choose how you want to prepare the technical checklist. You can evaluate with AI, fill manually, or combine both for full scrutiny.
              </p>
            </div>

            {/* Format selector */}
            {options.formats.length > 0 && (
              <div className="flex flex-col gap-1 text-xs">
                <ValidatedLabel htmlFor="tsc-format-select" className="font-semibold text-slate-700 dark:text-slate-300">
                  Checklist Format
                </ValidatedLabel>
                <ValidatedSelect
                  id="tsc-format-select"
                  className="rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-xs font-medium text-slate-900 shadow-sm transition focus:border-blue-600 focus:bg-white focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  value={selectedFormatId}
                  onChange={(e) => setSelectedFormatId(e.target.value)}
                  disabled={busy}
                >
                  {options.formats.map((f) => (
                    <option key={f.id} value={f.id}>
                      {f.name} ({f.question_count} questions)
                    </option>
                  ))}
                </ValidatedSelect>
              </div>
            )}
          </div>

          {/* Explanatory banner explaining the 3 options and the recommended flow */}
          <div className="mt-6 rounded-xl border border-blue-100 bg-blue-50/70 p-4 dark:border-blue-900/60 dark:bg-blue-950/40">
            <div className="flex items-start gap-3">
              <LightBulbIcon className="h-5 w-5 shrink-0 text-blue-600 dark:text-blue-400 mt-0.5" />
              <div className="text-xs">
                <h4 className="font-bold text-blue-900 dark:text-blue-200">
                  How to Prepare the Technical Scrutiny Checklist (TMC):
                </h4>
                <ul className="mt-1.5 space-y-1 text-slate-600 dark:text-slate-300 list-disc list-inside">
                  <li>
                    <strong className="text-slate-800 dark:text-slate-100">Option 1 (Cloud LLM AI):</strong> Evaluates verified tender documents automatically using configured AI to extract criteria and attach source page citations.
                  </li>
                  <li>
                    <strong className="text-slate-800 dark:text-slate-100">Option 2 (Manual Entry):</strong> Open the questionnaire grid directly to review tender documents side-by-side and record answers manually.
                  </li>
                  <li>
                    <strong className="text-blue-800 dark:text-blue-300">Option 3 (Recommended Combined Flow):</strong> Start with <strong>Cloud LLM</strong> to auto-populate facts and citations from documents, then inspect remaining unfilled or judgement-based questions directly in the grid to finalize the scrutiny.
                  </li>
                </ul>
              </div>
            </div>
          </div>

          {/* Two Option Cards Grid */}
          <div className="mt-6 grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Card 1: Cloud LLM */}
            <div className="flex flex-col justify-between rounded-xl border border-blue-200 bg-gradient-to-b from-blue-50/40 to-white p-5 shadow-sm transition hover:shadow-md dark:border-blue-900/60 dark:from-blue-950/20 dark:to-slate-900">
              <div>
                <div className="flex items-center justify-between">
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-blue-100 px-2.5 py-0.5 text-[11px] font-bold text-blue-800 dark:bg-blue-900/80 dark:text-blue-200">
                    <SparklesIcon className="h-3.5 w-3.5 text-blue-600 dark:text-blue-300" />
                    Option 1 · Cloud LLM
                  </span>
                  {activeProvider?.available ? (
                    <span className="text-[11px] font-semibold text-emerald-600 dark:text-emerald-400">
                      ● AI Ready
                    </span>
                  ) : (
                    <span className="text-[11px] font-semibold text-slate-400">
                      Not Configured
                    </span>
                  )}
                </div>

                <h3 className="mt-3 text-base font-bold text-slate-900 dark:text-white">
                  Automated Cloud LLM Scrutiny
                </h3>
                <p className="mt-1 text-xs text-slate-600 dark:text-slate-300 leading-relaxed">
                  Uses configured AI to read verified source PDFs (NIT, technical specifications, BOQ) and auto-fill questionnaire answers with clause citations.
                </p>

                <div className="mt-4 space-y-2 rounded-lg bg-slate-50 p-3 text-xs text-slate-600 dark:bg-slate-800/60 dark:text-slate-300 border border-slate-100 dark:border-slate-800">
                  <div className="flex items-center justify-between">
                    <span className="text-slate-500 dark:text-slate-400">AI Model Provider:</span>
                    {availableProviders.length > 1 ? (
                      <ValidatedSelect
                        className="rounded border border-slate-200 bg-white px-2 py-0.5 text-xs font-semibold text-slate-800 dark:border-slate-700 dark:bg-slate-700 dark:text-white"
                        value={selectedProvider}
                        onChange={(e) => setSelectedProvider(e.target.value)}
                        disabled={busy}
                      >
                        {availableProviders.map((p) => (
                          <option key={p.mode} value={p.mode}>
                            {p.name || p.mode.toUpperCase()} ({p.model})
                          </option>
                        ))}
                      </ValidatedSelect>
                    ) : (
                      <span className="font-semibold text-slate-800 dark:text-slate-200">
                        {activeProvider?.name || activeProvider?.mode?.toUpperCase() || "Gemini"} ({activeProvider?.model || "Standard"})
                      </span>
                    )}
                  </div>

                  <div className="flex items-center justify-between">
                    <span className="text-slate-500 dark:text-slate-400">Source Documents:</span>
                    <span className="font-semibold text-slate-800 dark:text-slate-200">
                      {pdfSources.length} verified PDF file{pdfSources.length === 1 ? "" : "s"}
                    </span>
                  </div>

                  {activeProvider?.reservation_paise ? (
                    <div className="flex items-center justify-between">
                      <span className="text-slate-500 dark:text-slate-400">Estimated Allowance:</span>
                      <span className="font-semibold text-slate-800 dark:text-slate-200">
                        ₹{(activeProvider.reservation_paise / 100).toFixed(2)}
                      </span>
                    </div>
                  ) : null}
                </div>

                {pdfSources.length === 0 && (
                  <div className="mt-3 flex items-start gap-2 rounded-lg bg-amber-50 p-2.5 text-[11px] text-amber-800 dark:bg-amber-950/60 dark:text-amber-300 border border-amber-200 dark:border-amber-900">
                    <ExclamationTriangleIcon className="h-4 w-4 shrink-0 text-amber-600 mt-0.5" />
                    <span>No PDF documents verified in Step 1. Upload PDFs in Step 1 to run AI scrutiny, or choose manual entry below.</span>
                  </div>
                )}
              </div>

              <div className="mt-6">
                <button
                  type="button"
                  className="w-full inline-flex items-center justify-center gap-2 rounded-xl bg-blue-600 px-4 py-2.5 text-xs font-bold text-white shadow-sm transition hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  disabled={busy || !canWrite || pdfSources.length === 0 || !activeProvider?.available}
                  onClick={() => void generate(activeProvider?.mode || "gemini")}
                >
                  <SparklesIcon className="h-4 w-4" />
                  {busy ? "Starting AI Evaluation…" : "Run AI Document Scrutiny"}
                </button>
              </div>
            </div>

            {/* Card 2: Manual Scrutiny Grid */}
            <div className="flex flex-col justify-between rounded-xl border border-slate-200 bg-white p-5 shadow-sm transition hover:shadow-md dark:border-slate-800 dark:bg-slate-900">
              <div>
                <div className="flex items-center justify-between">
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-slate-100 px-2.5 py-0.5 text-[11px] font-bold text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                    <PencilSquareIcon className="h-3.5 w-3.5 text-slate-500" />
                    Option 2 · Manual Scrutiny
                  </span>
                  <span className="text-[11px] font-semibold text-blue-600 dark:text-blue-400">
                    Always Available
                  </span>
                </div>

                <h3 className="mt-3 text-base font-bold text-slate-900 dark:text-white">
                  Manual Questionnaire Entry
                </h3>
                <p className="mt-1 text-xs text-slate-600 dark:text-slate-300 leading-relaxed">
                  Open the scrutiny grid directly to review tender documents side-by-side and enter answers, compliance verdicts, and scope notes manually.
                </p>

                <div className="mt-4 space-y-2 rounded-lg bg-slate-50 p-3 text-xs text-slate-600 dark:bg-slate-800/60 dark:text-slate-300 border border-slate-100 dark:border-slate-800">
                  <div className="flex items-center justify-between">
                    <span className="text-slate-500 dark:text-slate-400">Questions:</span>
                    <span className="font-semibold text-slate-800 dark:text-slate-200">
                      {selectedFormat?.question_count ?? 0} checklist questions
                    </span>
                  </div>

                  <div className="flex items-center justify-between">
                    <span className="text-slate-500 dark:text-slate-400">AI Tokens / Cost:</span>
                    <span className="font-semibold text-emerald-600 dark:text-emerald-400">
                      ₹0.00 (Zero Cost)
                    </span>
                  </div>

                  <div className="flex items-center justify-between">
                    <span className="text-slate-500 dark:text-slate-400">Access:</span>
                    <span className="font-semibold text-slate-800 dark:text-slate-200">
                      Immediate interactive grid
                    </span>
                  </div>
                </div>
              </div>

              <div className="mt-6">
                <button
                  type="button"
                  className="w-full inline-flex items-center justify-center gap-2 rounded-xl border border-slate-300 bg-white px-4 py-2.5 text-xs font-bold text-slate-800 shadow-sm transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:hover:bg-slate-700 disabled:opacity-50"
                  disabled={busy || !canWrite || !selectedFormat}
                  onClick={() => void generate("manual")}
                >
                  <PencilSquareIcon className="h-4 w-4" />
                  {busy ? "Opening Grid…" : "Open Manual Questionnaire Grid"}
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {job ? (
        <>
          {job.state !== "ready" && job.state !== "failed" && job.mode !== "manual" && (
            <div className="flex items-center justify-between rounded-2xl border border-blue-200 bg-blue-50/80 p-4 dark:border-blue-900/60 dark:bg-blue-950/40">
              <div className="flex items-center gap-3">
                <SparklesIcon className="h-5 w-5 animate-pulse text-blue-600 dark:text-blue-400" />
                <div>
                  <p className="text-xs font-bold text-blue-900 dark:text-blue-200">
                    AI Document Scrutiny Running…
                  </p>
                  <p className="text-[11px] text-blue-700 dark:text-blue-300">
                    Evaluating tender documents with {job.mode.toUpperCase()}. Answers will populate automatically as they complete. You can also edit and enter answers manually.
                  </p>
                </div>
              </div>
              <span className="inline-flex items-center gap-1.5 rounded-full bg-blue-100 px-3 py-1 text-xs font-bold text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                <ClockIcon className="h-3.5 w-3.5 animate-spin" />
                Processing
              </span>
            </div>
          )}

          {/* Header & Controls Card */}
          <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
            <div className="flex flex-wrap items-center justify-between gap-4">
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-xs font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
                    Tender Selection Checklist (TS-TSC)
                  </span>
                  <span className="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2.5 py-0.5 text-xs font-semibold text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                    {answeredCount}/{totalQuestions} Answered
                  </span>
                  {job.mode !== "manual" ? (
                    <span className="inline-flex items-center gap-1 rounded-full bg-blue-100 px-2.5 py-0.5 text-[11px] font-bold text-blue-800 dark:bg-blue-900/60 dark:text-blue-200">
                      <SparklesIcon className="h-3 w-3" />
                      Cloud LLM ({job.mode.toUpperCase()})
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2.5 py-0.5 text-[11px] font-semibold text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                      <PencilSquareIcon className="h-3 w-3 text-slate-500" />
                      Manual Scrutiny
                    </span>
                  )}
                </div>
                <h2 className="mt-1 text-xl font-bold text-slate-900 dark:text-white">
                  Technical Scrutiny Questionnaire
                </h2>
                <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
                  {job.snapshot.format_name} · Click any row to answer; answers save automatically as you move row by row.
                </p>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                {pdfSources.length > 0 && activeProvider?.available && (
                  <button
                    type="button"
                    className="inline-flex min-h-10 items-center justify-center gap-1.5 rounded-xl border border-blue-200 bg-blue-50 px-3.5 text-xs font-bold text-blue-700 shadow-sm transition hover:bg-blue-100 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-300"
                    disabled={busy || !canWrite}
                    onClick={() => void generate(activeProvider.mode)}
                    title="Run or re-run Cloud LLM evaluation on verified source documents"
                  >
                    <SparklesIcon className="h-4 w-4" />
                    {busy ? "Evaluating…" : "Run AI Scrutiny"}
                  </button>
                )}
                {has("tenders.tsc.export") && job.state === "ready" && job.answers.some((a) => a.answer.trim()) && (
                  <a
                    className="inline-flex min-h-10 items-center justify-center gap-1.5 rounded-xl border border-slate-300 bg-white px-3.5 text-xs font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                    href={`/api/tender-fms/${base}/${job.id}/document`}
                    download
                  >
                    <ArrowDownTrayIcon className="h-4 w-4" />
                    Download Word
                  </a>
                )}
                {onProceedToStep3 && (
                  <button
                    type="button"
                    className="inline-flex min-h-10 items-center justify-center gap-2 rounded-xl bg-blue-700 px-5 text-xs font-bold text-white shadow-sm transition hover:bg-blue-800"
                    onClick={() => void handleProceed()}
                  >
                    Save & Continue to Step 3
                    <ArrowRightIcon className="h-4 w-4" />
                  </button>
                )}
              </div>
            </div>

            {/* Progress bar */}
            <div className="mt-4">
              <div className="flex items-center justify-between text-xs text-slate-500 mb-1">
                <span className="font-medium">Completion Progress</span>
                <span className="font-bold text-blue-700 dark:text-blue-400">{progressPercent}%</span>
              </div>
              <div className="h-2 w-full rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
                <div
                  className="h-full bg-blue-600 transition-all duration-300 rounded-full"
                  style={{ width: `${progressPercent}%` }}
                />
              </div>
            </div>

            {/* Filter & Search Bar */}
            <div className="mt-4 flex flex-wrap items-center gap-3 border-t border-slate-100 pt-4 dark:border-slate-800">
              <div className="relative min-w-60 flex-1">
                <MagnifyingGlassIcon className="pointer-events-none absolute left-3 top-3 h-4 w-4 text-slate-400" />
                <ValidatedInput
                  className="w-full rounded-xl border border-slate-200 bg-slate-50 py-2 pl-9 pr-3 text-xs text-slate-900 transition focus:border-blue-600 focus:bg-white focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                  placeholder="Search questions or keywords…"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>

              {sections.length > 1 && (
                <div className="w-48">
                  <ValidatedSelect
                    className="w-full rounded-xl border border-slate-200 bg-slate-50 py-2 px-3 text-xs text-slate-900 transition focus:border-blue-600 focus:bg-white focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white"
                    value={sectionFilter}
                    onChange={(e) => setSectionFilter(e.target.value)}
                  >
                    <option value="all">All Sections ({sections.length})</option>
                    {sections.map((sec) => (
                      <option key={sec} value={sec}>
                        {formatSectionName(sec)}
                      </option>
                    ))}
                  </ValidatedSelect>
                </div>
              )}

              <div className="flex items-center gap-1 rounded-xl border border-slate-200 bg-slate-50 p-1 dark:border-slate-800 dark:bg-slate-800">
                <button
                  type="button"
                  className={`rounded-lg px-2.5 py-1 text-xs font-semibold transition ${
                    statusFilter === "all"
                      ? "bg-white text-blue-700 shadow-sm dark:bg-slate-700 dark:text-blue-300"
                      : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
                  }`}
                  onClick={() => setStatusFilter("all")}
                >
                  All ({totalQuestions})
                </button>
                <button
                  type="button"
                  className={`rounded-lg px-2.5 py-1 text-xs font-semibold transition ${
                    statusFilter === "unfilled"
                      ? "bg-white text-blue-700 shadow-sm dark:bg-slate-700 dark:text-blue-300"
                      : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
                  }`}
                  onClick={() => setStatusFilter("unfilled")}
                >
                  Needs Answer ({totalQuestions - answeredCount})
                </button>
                <button
                  type="button"
                  className={`rounded-lg px-2.5 py-1 text-xs font-semibold transition ${
                    statusFilter === "answered"
                      ? "bg-white text-blue-700 shadow-sm dark:bg-slate-700 dark:text-blue-300"
                      : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
                  }`}
                  onClick={() => setStatusFilter("answered")}
                >
                  Answered ({answeredCount})
                </button>
              </div>
            </div>
          </div>

          {/* Centralized Questionnaire Grid Table */}
          <div className="rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900 overflow-hidden">
            <div className="overflow-x-auto">
              <table className="tsc-grid-table w-full table-fixed text-left text-xs border-collapse">
                <colgroup>
                  <col style={{ width: "56px" }} />
                  <col style={{ width: "36%" }} />
                  <col style={{ width: "50%" }} />
                  <col style={{ width: "120px" }} />
                </colgroup>
                <thead>
                  <tr className="border-b border-slate-200 bg-slate-50/80 font-bold uppercase tracking-wider text-slate-600 dark:border-slate-800 dark:bg-slate-800/80 dark:text-slate-300">
                    <th scope="col" className="px-4 py-3.5 text-center">
                      #
                    </th>
                    <th scope="col" className="px-4 py-3.5">
                      Tender Specification & Requirements
                    </th>
                    <th scope="col" className="px-4 py-3.5">
                      Tender Working Answer
                    </th>
                    <th scope="col" className="px-4 py-3.5 text-right">
                      Status
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 dark:divide-slate-800/60">
                  {filteredQuestions.length === 0 ? (
                    <tr>
                      <td colSpan={4} className="p-8 text-center text-slate-500">
                        No questions match the current filter or search criteria.
                      </td>
                    </tr>
                  ) : (
                    filteredQuestions.map((q, idx) => {
                      const prevQ = idx > 0 ? filteredQuestions[idx - 1] : null;
                      const showSectionDivider = sectionFilter === "all" && (!prevQ || prevQ.section !== q.section);
                      const isRowActive = activeId === q.id;
                      const answerDraft = workingDrafts[q.id]?.answer ?? "";
                      const currentStatus = workingDrafts[q.id]?.status ?? "needs_review";
                      const saveState = rowStatus[q.id] || "idle";

                      // Calculate vertical expansion dynamically based on text length and line breaks
                      const lineCount = (answerDraft.match(/\n/g) || []).length + 1;
                      const estimatedLines = Math.max(lineCount, Math.ceil((answerDraft.length || 1) / 55));
                      const dynamicRows = Math.min(10, Math.max(isRowActive ? 3 : 2, estimatedLines));

                      return (
                        <>
                          {showSectionDivider && (
                            <tr key={`sec-${q.section}`} className="border-y border-slate-200 bg-slate-100/80 dark:border-slate-800 dark:bg-slate-800/90 font-bold uppercase tracking-wider text-slate-700 dark:text-slate-200">
                              <td colSpan={4} className="px-4 py-2 text-[11px]">
                                {formatSectionName(q.section)}
                              </td>
                            </tr>
                          )}
                          <tr
                            key={q.id}
                            className={`transition-colors ${
                              isRowActive
                                ? "bg-blue-50/40 ring-1 ring-inset ring-blue-500/30 dark:bg-blue-950/20"
                                : "hover:bg-slate-50/70 dark:hover:bg-slate-800/40"
                            }`}
                          >
                            {/* # Number badge */}
                            <td className="px-4 py-3 text-center align-top">
                              <span
                                className={`inline-flex h-6 w-6 items-center justify-center rounded-md text-[11px] font-bold ${
                                  isRowActive
                                    ? "bg-blue-600 text-white"
                                    : "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300"
                                }`}
                              >
                                {q.printed_number || idx + 1}
                              </span>
                            </td>

                            {/* Left: Question title and category */}
                            <td className="px-4 py-3 align-top">
                              <p className="text-xs font-semibold text-slate-900 dark:text-white leading-relaxed">
                                {q.question}
                              </p>
                              <div className="mt-1 flex flex-wrap items-center gap-1.5">
                                <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wider text-slate-600 dark:bg-slate-800 dark:text-slate-400">
                                  {formatSectionName(q.section)}
                                </span>
                                {q.manual_review && (
                                  <span className="rounded bg-amber-50 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700 border border-amber-200 dark:bg-amber-950 dark:text-amber-300">
                                    Management Judgement
                                  </span>
                                )}
                                {job.answers.find((a) => a.question_id === q.id)?.citations?.length ? (
                                  <span className="inline-flex items-center gap-1 rounded bg-blue-50 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700 border border-blue-200 dark:bg-blue-950 dark:text-blue-300 dark:border-blue-800">
                                    <SparklesIcon className="h-3 w-3" />
                                    AI Cited ({job.answers.find((a) => a.question_id === q.id)?.citations?.length} clause{job.answers.find((a) => a.question_id === q.id)?.citations?.length === 1 ? "" : "s"})
                                  </span>
                                ) : null}
                              </div>
                              {job.answers.find((a) => a.question_id === q.id)?.citations?.length ? (
                                <details className="mt-1.5 text-[11px] text-slate-500">
                                  <summary className="cursor-pointer font-medium hover:text-blue-600 dark:hover:text-blue-400">
                                    View source clause citation ({job.answers.find((a) => a.question_id === q.id)?.citations.length})
                                  </summary>
                                  <div className="mt-1 space-y-1 rounded-lg bg-slate-50 p-2 text-[10px] text-slate-700 dark:bg-slate-800 dark:text-slate-300 border border-slate-200 dark:border-slate-700">
                                    {job.answers.find((a) => a.question_id === q.id)?.citations.map((c, i) => (
                                      <div key={i} className="border-l-2 border-blue-500 pl-2">
                                        <p className="font-semibold text-blue-800 dark:text-blue-300">Page {c.page}:</p>
                                        <blockquote className="italic whitespace-pre-wrap">&ldquo;{c.quote}&rdquo;</blockquote>
                                      </div>
                                    ))}
                                  </div>
                                </details>
                              ) : null}
                            </td>

                            {/* Right: Working Answer Textarea (Vertical auto-expand, NO horizontal scroll) */}
                            <td className="px-4 py-3 align-top">
                              <div className="relative">
                                <ValidatedTextarea
                                  rows={dynamicRows}
                                  className={`tsc-grid-textarea w-full resize-y overflow-x-hidden overflow-y-auto whitespace-pre-wrap break-words rounded-xl border p-2.5 text-xs text-slate-900 transition-all focus:bg-white focus:outline-none dark:text-white dark:bg-slate-800/90 ${
                                    isRowActive
                                      ? "border-blue-600 ring-2 ring-blue-100 dark:border-blue-500 dark:ring-blue-950"
                                      : "border-slate-200 dark:border-slate-700 hover:border-slate-300"
                                  }`}
                                  style={{
                                    overflowX: "hidden",
                                    resize: "vertical",
                                    whiteSpace: "pre-wrap",
                                    wordBreak: "break-word",
                                    overflowWrap: "break-word",
                                  }}
                                  placeholder="Enter answer or specification…"
                                  value={answerDraft}
                                  disabled={!canWrite}
                                  onFocus={() => setActiveId(q.id)}
                                  onChange={(e) => handleTextChange(q.id, e.target.value)}
                                  onBlur={() => saveQuestion(q.id)}
                                />
                              </div>
                            </td>

                            {/* Far Right: Status selector & Auto-save confirmation */}
                            <td className="px-4 py-3 align-top text-right">
                              <div className="flex flex-col items-end gap-1.5">
                                <ValidatedSelect
                                  className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-[11px] font-semibold text-slate-700 shadow-sm focus:border-blue-600 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
                                  value={currentStatus}
                                  disabled={!canWrite}
                                  onChange={(e) => handleStatusChange(q.id, e.target.value as Answer["status"])}
                                  onBlur={() => saveQuestion(q.id)}
                                >
                                  <option value="answered">Answered</option>
                                  <option value="needs_review">Needs Review</option>
                                  <option value="not_found">Not Found</option>
                                  <option value="conflicting">Conflicting</option>
                                </ValidatedSelect>

                                {/* Auto-save indicator */}
                                <div className="min-h-4">
                                  {saveState === "saving" ? (
                                    <span className="inline-flex items-center gap-1 text-[11px] font-semibold text-blue-600 animate-pulse">
                                      <ClockIcon className="h-3.5 w-3.5" />
                                      Saving…
                                    </span>
                                  ) : saveState === "saved" ? (
                                    <span className="inline-flex items-center gap-1 text-[11px] font-semibold text-emerald-600 dark:text-emerald-400">
                                      <CheckCircleIcon className="h-3.5 w-3.5" />
                                      Saved
                                    </span>
                                  ) : saveState === "error" ? (
                                    <button
                                      type="button"
                                      className="inline-flex items-center gap-1 text-[11px] font-semibold text-red-600 hover:text-red-700 underline focus:outline-none"
                                      onClick={() => saveQuestion(q.id)}
                                      title="Click to retry saving this answer"
                                    >
                                      <ExclamationTriangleIcon className="h-3.5 w-3.5" />
                                      Retry
                                    </button>
                                  ) : answerDraft.trim().length > 0 ? (
                                    <span className="inline-flex items-center gap-1 text-[11px] font-semibold text-emerald-600 dark:text-emerald-400">
                                      <CheckCircleIcon className="h-3.5 w-3.5" />
                                      Saved
                                    </span>
                                  ) : null}
                                </div>
                              </div>
                            </td>
                          </tr>
                        </>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>

            {/* Bottom Summary Bar */}
            <div className="flex flex-wrap items-center justify-between gap-4 border-t border-slate-200 bg-slate-50/70 p-4 dark:border-slate-800 dark:bg-slate-900/80">
              <div className="flex items-center gap-4 text-xs">
                <span className="font-bold text-slate-700 dark:text-slate-300">
                  Total Questions: {totalQuestions}
                </span>
                <span className="text-emerald-700 dark:text-emerald-400 font-semibold">
                  ✓ {answeredCount} Answered
                </span>
                <span className="text-slate-500">
                  {totalQuestions - answeredCount} Remaining
                </span>
              </div>

              {onProceedToStep3 && (
                <button
                  type="button"
                  className="inline-flex min-h-10 items-center justify-center gap-2 rounded-xl bg-blue-700 px-5 text-xs font-bold text-white shadow-sm transition hover:bg-blue-800"
                  onClick={() => void handleProceed()}
                >
                  Submit Questionnaire & Continue to Step 3
                  <ArrowRightIcon className="h-4 w-4" />
                </button>
              )}
            </div>
          </div>
        </>
      ) : null}
    </section>
  );
}
