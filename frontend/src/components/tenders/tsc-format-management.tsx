"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import {
  PlusIcon,
  DocumentDuplicateIcon,
  DocumentTextIcon,
  ArrowLeftIcon,
  EyeIcon,
  PencilSquareIcon,
  ArchiveBoxIcon,
  ArrowPathIcon,
  TrashIcon,
  ChevronUpIcon,
  ChevronDownIcon,
  ClipboardDocumentListIcon,
  CheckIcon,
  XMarkIcon,
  ArrowDownTrayIcon,
  SparklesIcon,
  UserIcon,
} from "@heroicons/react/24/outline";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { actionButtonClass } from "@/components/ui/action-button";
import { DataGrid, type GridColumn } from "@/components/ui/data-grid";
import {
  ValidatedForm,
  ValidatedInput,
  ValidatedLabel,
  ValidatedSelect,
  ValidatedTextarea,
} from "@/components/ui/validated-controls";
import { DocumentPDFViewer } from "@/components/documents/document-pdf-viewer";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { createRecordId } from "@/lib/record-id";
import {
  duplicateQuestions,
  type TSCFormat,
  type FormatQuestion,
} from "./tsc-format-model";

const url = "/api/settings/tsc-formats";
const primaryBtn = actionButtonClass("primary");
const secondaryBtn = actionButtonClass("secondary");
const iconBtn = actionButtonClass("secondary", true);
const controlClass =
  "min-h-10 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 transition focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100";

async function request<T>(path = "", init?: RequestInit): Promise<T> {
  const response = await fetch(url + path, { cache: "no-store", ...init });
  const body = await response.json().catch(() => ({
    detail: "The format service returned an unreadable response",
  }));
  if (!response.ok) throw new Error(body.detail || "TSC formats could not be loaded");
  return body as T;
}

export function TSCFormatManagement() {
  const user = useAuthenticatedUser();
  const writable =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("settings.tsc_formats.write");

  const [formats, setFormats] = useState<TSCFormat[]>([]);
  const [draft, setDraft] = useState<TSCFormat | null>(null);

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [statusTogglingId, setStatusTogglingId] = useState<string | null>(null);

  // Question filtering & inline adding (in editor view)
  const [questionSearch, setQuestionSearch] = useState("");
  const [activeSection, setActiveSection] = useState<string>("all");
  const [newSection, setNewSection] = useState("General");
  const [newQuestionText, setNewQuestionText] = useState("");
  const [newManualReview, setNewManualReview] = useState(false);

  // In-row editing
  const [editingQuestionId, setEditingQuestionId] = useState<string | null>(null);
  const [editText, setEditText] = useState("");
  const [editSection, setEditSection] = useState("");
  const [editNumber, setEditNumber] = useState("");
  const [editManual, setEditManual] = useState(false);

  // Bulk paste modal & preview drawer
  const [pasteModalOpen, setPasteModalOpen] = useState(false);
  const [pasteRawText, setPasteRawText] = useState("");
  const [pasteDefaultSection, setPasteDefaultSection] = useState("Technical");
  const [previewFormat, setPreviewFormat] = useState<TSCFormat | null>(null);

  // Undo support
  const [removed, setRemoved] = useState<{ question: FormatQuestion; index: number } | null>(null);

  const quickAddInputRef = useRef<HTMLInputElement>(null);
  const selectionTracker = useRef(0);
  const busyRef = useRef(false);

  // Load formats list
  const loadFormats = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    try {
      const data = await request<{ items: TSCFormat[] }>("", { signal });
      setFormats(data.items);
      return data.items;
    } catch (e) {
      if (!signal?.aborted) setError(e instanceof Error ? e.message : "Failed to load formats");
      return [];
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, []);

  useEffect(() => {
    const c = new AbortController();
    const initial = setTimeout(() => void loadFormats(c.signal), 0);
    return () => { clearTimeout(initial); c.abort(); };
  }, [loadFormats]);

  // Warn before leaving with unsaved changes
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  const questions = draft?.questions || [];
  const duplicates = duplicateQuestions(questions);

  // Unique sections in current format
  const sections = Array.from(new Set(questions.map((q) => q.section.trim()).filter(Boolean)));
  if (!sections.includes("General")) sections.unshift("General");

  // Filtered questions in workspace
  const visibleQuestions = questions
    .map((question, index) => ({ question, index }))
    .filter(({ question }) => {
      const matchSection = activeSection === "all" || question.section === activeSection;
      const matchSearch =
        !questionSearch.trim() ||
        `${question.question} ${question.section} ${question.printed_number}`
          .toLowerCase()
          .includes(questionSearch.toLowerCase());
      return matchSection && matchSearch;
    });

  function canLeave() {
    return !dirty || window.confirm("Discard unsaved changes to this TSC format?");
  }

  function markChange(patch: Partial<TSCFormat>) {
    setDraft((prev) => (prev ? { ...prev, ...patch } : prev));
    setDirty(true);
    setNotice("");
  }

  async function openFormat(id: string) {
    if (saving) return;
    if (!canLeave()) return;

    const currentReq = ++selectionTracker.current;
    setLoading(true);
    setError("");
    setNotice("");
    try {
      const full = await request<TSCFormat>(`/${id}`);
      if (selectionTracker.current !== currentReq) return;
      setDraft(full);
      setDirty(false);
      setRemoved(null);
      setEditingQuestionId(null);
      setActiveSection("all");
      setQuestionSearch("");
      window.scrollTo({ top: 0, behavior: "smooth" });
    } catch (e) {
      if (selectionTracker.current === currentReq) {
        setError(e instanceof Error ? e.message : "Could not open format");
      }
    } finally {
      if (selectionTracker.current === currentReq) setLoading(false);
    }
  }

  function createNewFormat(copy = false) {
    if (saving || !canLeave()) return;
    selectionTracker.current++;
    const baseQuestions = copy && draft ? draft.questions?.map((q) => ({ ...q })) || [] : [];
    const newDraft: TSCFormat = {
      id: "",
      name: copy && draft ? `${draft.name} (Copy)` : "New EPC Checklist Format",
      description: copy && draft ? draft.description : "",
      purpose: copy && draft ? draft.purpose || "Generic" : "Generic",
      is_active: true,
      version: 0,
      question_count: baseQuestions.length,
      questions: baseQuestions,
    };
    setDraft(newDraft);
    setDirty(copy);
    setError("");
    setNotice(copy ? "Created duplicate draft format." : "Created new format draft.");
    setRemoved(null);
    setEditingQuestionId(null);
    setActiveSection("all");
    setQuestionSearch("");
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function returnToGrid() {
    if (!canLeave()) return;
    setDraft(null);
    setDirty(false);
    setError("");
    setNotice("");
    setRemoved(null);
  }

  // --- Lightning Quick Add Question (Zero Modals, Part of Grid) ---
  function handleQuickAddQuestion(e?: React.FormEvent) {
    if (e) e.preventDefault();
    if (!writable || !draft) return;

    const trimmed = newQuestionText.trim();
    if (!trimmed) {
      // Nothing typed - do not block or show error
      return;
    }
    if (trimmed.length < 3) {
      setError("Question text must be at least 3 characters.");
      return;
    }

    const sec = (newSection.trim() || (activeSection !== "all" ? activeSection : "General")).trim();
    const newQ: FormatQuestion = {
      id: createRecordId(),
      section: sec,
      printed_number: String(questions.length + 1),
      question: trimmed,
      manual_review: newManualReview,
    };

    markChange({
      questions: [...questions, newQ],
      question_count: questions.length + 1,
    });

    setNewQuestionText("");
    setError("");
    requestAnimationFrame(() => {
      quickAddInputRef.current?.focus();
    });
  }

  // --- Inline Row Editing ---
  function startInlineEdit(q: FormatQuestion) {
    setEditingQuestionId(q.id);
    setEditText(q.question);
    setEditSection(q.section);
    setEditNumber(q.printed_number);
    setEditManual(Boolean(q.manual_review));
  }

  function commitInlineEdit(id: string) {
    if (!draft || !editText.trim()) return;
    const updated = questions.map((q) =>
      q.id === id
        ? {
            ...q,
            question: editText.trim(),
            section: editSection.trim() || "General",
            printed_number: editNumber.trim(),
            manual_review: editManual,
          }
        : q
    );
    markChange({ questions: updated });
    setEditingQuestionId(null);
  }

  function toggleQuestionManualMode(index: number) {
    if (!draft || !writable) return;
    const updated = [...questions];
    updated[index] = {
      ...updated[index],
      manual_review: !updated[index].manual_review,
    };
    markChange({ questions: updated });
  }

  function duplicateQuestion(index: number) {
    if (!draft || !writable) return;
    const target = questions[index];
    const copy: FormatQuestion = {
      ...target,
      id: createRecordId(),
      printed_number: `${target.printed_number || index + 1}a`,
      question: `${target.question} (Copy)`,
    };
    const next = [...questions];
    next.splice(index + 1, 0, copy);
    markChange({ questions: next, question_count: next.length });
    setNotice("Question duplicated.");
  }

  function moveQuestion(index: number, delta: number) {
    if (!draft || !writable) return;
    const targetIndex = index + delta;
    if (targetIndex < 0 || targetIndex >= questions.length) return;
    const next = [...questions];
    [next[index], next[targetIndex]] = [next[targetIndex], next[index]];
    markChange({ questions: next });
  }

  function removeQuestion(index: number) {
    if (!draft || !writable) return;
    const target = questions[index];
    setRemoved({ question: target, index });
    const next = questions.filter((_, i) => i !== index);
    markChange({ questions: next, question_count: next.length });
    setNotice("Question removed.");
  }

  function undoRemoval() {
    if (!removed || !draft) return;
    const next = [...questions];
    next.splice(removed.index, 0, removed.question);
    markChange({ questions: next, question_count: next.length });
    setRemoved(null);
    setNotice("Question restored.");
  }

  // --- Bulk Paste Questions ---
  function handleImportPastedQuestions() {
    if (!draft || !pasteRawText.trim()) return;
    const lines = pasteRawText
      .split(/\r?\n/)
      .map((l) => l.trim())
      .filter((l) => l.length > 2);

    if (!lines.length) {
      setPasteModalOpen(false);
      return;
    }

    const parsed: FormatQuestion[] = lines.map((line, idx) => {
      let sec = pasteDefaultSection;
      let text = line;

      const prefixMatch = line.match(/^\[?([A-Za-z0-9\s&/-]{3,30})\]?\s*[:\-]\s*(.+)$/);
      if (prefixMatch) {
        sec = prefixMatch[1].trim();
        text = prefixMatch[2].trim();
      } else {
        text = text.replace(/^(\d+[\.\)]|\([a-zA-Z0-9]+\))\s*/, "");
      }

      return {
        id: createRecordId(),
        section: sec || "General",
        printed_number: String(questions.length + idx + 1),
        question: text,
        manual_review: false,
      };
    });

    const next = [...questions, ...parsed];
    markChange({ questions: next, question_count: next.length });
    setPasteRawText("");
    setPasteModalOpen(false);
    setNotice(`Successfully imported ${parsed.length} questions.`);
  }

  // --- Save Format to API ---
  async function saveCurrentFormat() {
    if (!draft || busyRef.current || !writable) return;
    if (draft.name.trim().length < 3) {
      setError("Format name must be at least 3 characters.");
      return;
    }
    if (!questions.length) {
      setError("Add at least one question before saving.");
      return;
    }
    if (duplicates.size > 0) {
      setError("Resolve duplicate question wording before saving this format.");
      return;
    }

    busyRef.current = true;
    setSaving(true);
    setError("");
    setNotice("");
    try {
      const payload = {
        name: draft.name.trim(),
        description: draft.description || "",
        purpose: draft.purpose || "Generic",
        is_active: draft.is_active,
        expected_version: draft.version,
        questions,
      };

      const result = await request<TSCFormat>(draft.id ? `/${draft.id}` : "", {
        method: draft.id ? "PATCH" : "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      setDraft(result);
      setFormats((prev) =>
        [...prev.filter((f) => f.id !== result.id), result].sort((a, b) =>
          a.name.localeCompare(b.name)
        )
      );
      setDirty(false);
      setRemoved(null);
      setNotice(`Format "${result.name}" saved successfully (Revision ${result.version}).`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Format could not be saved");
    } finally {
      busyRef.current = false;
      setSaving(false);
    }
  }

  // Keyboard shortcut Ctrl+S / Cmd+S to save
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "s") {
        e.preventDefault();
        if (draft && dirty && !saving && writable) {
          void saveCurrentFormat();
        }
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  });

  // --- Toggle Active Status ---
  async function toggleStatus(format: TSCFormat) {
    if (busyRef.current || !writable) return;
    busyRef.current = true;
    setStatusTogglingId(format.id);
    setError("");
    setNotice("");
    try {
      const latest = await request<TSCFormat>(`/${format.id}`);
      const result = await request<TSCFormat>(`/${format.id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: latest.name,
          description: latest.description,
          purpose: latest.purpose || "Generic",
          is_active: !latest.is_active,
          expected_version: latest.version,
          questions: latest.questions,
        }),
      });

      setFormats((prev) => prev.map((f) => (f.id === result.id ? result : f)));
      if (draft && draft.id === result.id) {
        setDraft((prev) => (prev ? { ...prev, is_active: result.is_active } : prev));
      }
      setNotice(
        `Format "${result.name}" ${result.is_active ? "activated" : "deactivated"}.`
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not toggle status");
    } finally {
      busyRef.current = false;
      setStatusTogglingId(null);
    }
  }

  // Standard DataGrid columns for the master list
  const columns: GridColumn<TSCFormat>[] = [
    {
      key: "name",
      label: "Format Name",
      value: (f) => f.name,
      render: (f) => (
        <div>
          <button
            type="button"
            className="text-left font-bold text-slate-900 transition hover:text-blue-700 hover:underline dark:text-white dark:hover:text-blue-400"
            onClick={() => void openFormat(f.id)}
          >
            {f.name}
          </button>
          {f.description ? (
            <p className="mt-0.5 max-w-xl text-xs text-slate-500 line-clamp-1">
              {f.description}
            </p>
          ) : null}
        </div>
      ),
    },
    {
      key: "purpose",
      label: "Purpose / Domain",
      value: (f) => f.purpose || "Generic",
      render: (f) => (
        <span className="rounded-lg bg-slate-100 px-2.5 py-1 text-xs font-semibold text-slate-700 dark:bg-slate-800 dark:text-slate-300">
          {f.purpose || "Generic"}
        </span>
      ),
    },
    {
      key: "questions",
      label: "Questions",
      value: (f) => f.question_count,
      render: (f) => (
        <span className="rounded-lg bg-blue-50 px-2.5 py-1 text-xs font-bold text-blue-800 dark:bg-blue-950 dark:text-blue-300">
          {f.question_count} questions
        </span>
      ),
    },
    {
      key: "version",
      label: "Revision",
      value: (f) => f.version,
      render: (f) => (
        <span className="font-mono text-xs text-slate-600 dark:text-slate-400">
          Rev {f.version}
        </span>
      ),
    },
    {
      key: "created",
      label: "Created On",
      value: (f) =>
        f.created_at && !Number.isNaN(Date.parse(f.created_at))
          ? new Intl.DateTimeFormat("en-GB", {
              day: "2-digit",
              month: "short",
              year: "numeric",
            }).format(new Date(f.created_at))
          : "—",
    },
    {
      key: "status",
      label: "Status",
      value: (f) => (f.is_active ? "Active" : "Inactive"),
      render: (f) => (
        <span
          className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-bold ${
            f.is_active
              ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300"
              : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400"
          }`}
        >
          {f.is_active ? "Active" : "Inactive"}
        </span>
      ),
    },
  ];

  return (
    <div className="mx-auto w-full max-w-[1500px] space-y-5 p-4 sm:p-6" aria-label="TSC Checklist Formats">
      {/* Top Banner Alert */}
      {notice && (
        <div role="status" className="tsc-alert is-success">
          <span>{notice}</span>
          <button
            type="button"
            className="text-xs font-bold text-emerald-800 underline dark:text-emerald-200"
            onClick={() => setNotice("")}
          >
            Dismiss
          </button>
        </div>
      )}
      {error && (
        <div role="alert" className="tsc-alert is-error">
          <span>{error}</span>
          <button
            type="button"
            className="text-xs font-bold text-red-800 underline dark:text-red-200"
            onClick={() => setError("")}
          >
            Dismiss
          </button>
        </div>
      )}
      {removed && (
        <div role="status" className="tsc-alert is-warning">
          <span>Question removed from this draft format.</span>
          <button
            type="button"
            className={secondaryBtn + " text-xs py-1 px-3"}
            disabled={saving}
            onClick={undoRemoval}
          >
            Undo Removal
          </button>
        </div>
      )}

      {/* Screen 1: Master List View with DataGrid (Default Entry) */}
      {!draft ? (
        <section className="space-y-4">
          <header className="flex flex-wrap items-end justify-between gap-3 border-b border-slate-200 pb-4 dark:border-slate-800">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
                Global Masters · Tender Evaluation
              </p>
              <h1 className="text-2xl font-extrabold text-slate-900 dark:text-white">
                TSC Formats
              </h1>
              <p className="mt-1 text-sm text-slate-600 dark:text-slate-400">
                Standard technical, commercial, and financial tender scrutiny checklists used for bid qualification.
              </p>
            </div>
            {writable && (
              <button
                type="button"
                className={primaryBtn + " gap-2 px-4 py-2 text-sm font-bold"}
                onClick={() => createNewFormat(false)}
              >
                <PlusIcon className="h-4 w-4" />
                New format
              </button>
            )}
          </header>

          <DataGrid
            label="TSC format directory"
            rows={formats}
            loading={loading}
            searchable={true}
            paginate={true}
            status={(f) => (f.is_active ? "Active" : "Inactive")}
            initialSort="name"
            stateKey="tsc_formats"
            columns={columns}
            onRowClick={(f) => void openFormat(f.id)}
            actions={(f) => (
              <div
                className="flex items-center justify-end gap-1"
                onClick={(e) => e.stopPropagation()}
              >
                <button
                  type="button"
                  className={iconBtn}
                  aria-label={`View ${f.name}`}
                  title="View document preview"
                  onClick={() => setPreviewFormat(f)}
                >
                  <EyeIcon className="h-4 w-4" />
                </button>
                {writable && (
                  <>
                    <button
                      type="button"
                      className={iconBtn}
                      aria-label={`Edit ${f.name}`}
                      title="Edit format"
                      onClick={() => void openFormat(f.id)}
                    >
                      <PencilSquareIcon className="h-4 w-4" />
                    </button>
                    <button
                      type="button"
                      className={iconBtn}
                      aria-label={`Make a copy of ${f.name}`}
                      title="Duplicate format"
                      onClick={() => {
                        void openFormat(f.id).then(() => createNewFormat(true));
                      }}
                    >
                      <DocumentDuplicateIcon className="h-4 w-4" />
                    </button>
                    <button
                      type="button"
                      className={actionButtonClass(f.is_active ? "danger" : "success", true)}
                      aria-label={`${f.is_active ? "Deactivate" : "Activate"} ${f.name}`}
                      title={f.is_active ? "Deactivate format" : "Activate format"}
                      disabled={statusTogglingId === f.id}
                      onClick={() => void toggleStatus(f)}
                    >
                      {f.is_active ? (
                        <ArchiveBoxIcon className="h-4 w-4" />
                      ) : (
                        <ArrowPathIcon className="h-4 w-4" />
                      )}
                    </button>
                  </>
                )}
              </div>
            )}
          />
        </section>
      ) : (
        /* Screen 2: Low-Click Format Workspace (Opened on Row Click or + New) */
        <section className="space-y-5 rounded-2xl border border-slate-200 bg-white p-5 dark:border-slate-800 dark:bg-slate-900">
          {/* Top Return & Save Toolbar */}
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 pb-4 dark:border-slate-800">
            <div className="flex items-center gap-3">
              <button
                type="button"
                className={secondaryBtn + " gap-1.5 px-3 py-1.5 text-xs font-bold"}
                onClick={returnToGrid}
                title="Return to formats list"
              >
                <ArrowLeftIcon className="h-4 w-4" />
                All TSC formats
              </button>
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-xs font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
                    {draft.id ? `Revision ${draft.version}` : "New Format Draft"}
                  </span>
                  {dirty && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-900 dark:bg-amber-950 dark:text-amber-300">
                      ● Unsaved changes
                    </span>
                  )}
                </div>
                <h2 className="text-xl font-bold text-slate-900 dark:text-white">
                  {draft.name || "Untitled Format"}
                </h2>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              {draft.id && (
                <>
                  <a
                    href={`${url}/${draft.id}/template.docx`}
                    download
                    className={secondaryBtn + " gap-1 text-xs py-1.5 px-3 min-h-9"}
                    title="Download Word template"
                  >
                    <ArrowDownTrayIcon className="h-4 w-4" />
                    Word
                  </a>
                  <button
                    type="button"
                    className={secondaryBtn + " gap-1 text-xs py-1.5 px-3 min-h-9"}
                    title="View document PDF"
                    onClick={() => setPreviewFormat(draft)}
                  >
                    <EyeIcon className="h-4 w-4" />
                    Preview
                  </button>
                  {writable && (
                    <button
                      type="button"
                      className={secondaryBtn + " gap-1 text-xs py-1.5 px-3 min-h-9"}
                      title="Duplicate this format"
                      onClick={() => createNewFormat(true)}
                    >
                      <DocumentDuplicateIcon className="h-4 w-4" />
                      Copy
                    </button>
                  )}
                </>
              )}

              {writable && (
                <button
                  type="button"
                  className={primaryBtn + " gap-1.5 text-xs py-1.5 px-4 min-h-9 font-bold"}
                  disabled={saving || !questions.length || (!dirty && Boolean(draft.id))}
                  onClick={() => void saveCurrentFormat()}
                  title="Save format (Ctrl+S / ⌘S)"
                >
                  <CheckIcon className="h-4 w-4" />
                  {saving ? "Saving…" : "Save Format"}
                </button>
              )}
            </div>
          </div>

          {/* Unified Metadata Row (Directly visible, no tabs!) */}
          <div className="grid gap-4 rounded-xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-950 sm:grid-cols-12">
            <div className="sm:col-span-6">
              <ValidatedLabel className="text-xs font-bold text-slate-700 dark:text-slate-300">
                Format Title
                <ValidatedInput
                  required
                  minLength={3}
                  maxLength={120}
                  className={controlClass + " mt-1 font-semibold"}
                  value={draft.name}
                  disabled={!writable || saving}
                  onChange={(e) => markChange({ name: e.target.value })}
                  placeholder="e.g. EPC Works — Standard Technical Evaluation"
                />
              </ValidatedLabel>
            </div>

            <div className="sm:col-span-3">
              <ValidatedLabel className="text-xs font-bold text-slate-700 dark:text-slate-300">
                Purpose / Domain
                <ValidatedSelect
                  className={controlClass + " mt-1"}
                  value={draft.purpose || "Generic"}
                  disabled={!writable || saving}
                  onChange={(e) => markChange({ purpose: e.target.value })}
                >
                  <option value="Generic">Generic EPC</option>
                  <option value="Electrical">Electrical Transmission & Substation</option>
                  <option value="Solar">Solar Power Plant</option>
                  <option value="Civil">Civil & Structural Works</option>
                  <option value="Instrumentation">Instrumentation & Control</option>
                </ValidatedSelect>
              </ValidatedLabel>
            </div>

            <div className="flex flex-col justify-end sm:col-span-3">
              <ValidatedLabel className="flex min-h-10 cursor-pointer items-center gap-2 text-xs font-semibold text-slate-700 dark:text-slate-300">
                <ValidatedInput
                  type="checkbox"
                  checked={draft.is_active}
                  disabled={!writable || saving}
                  onChange={(e) => markChange({ is_active: e.target.checked })}
                  className="h-4 w-4 accent-blue-600"
                />
                Active for tenders
              </ValidatedLabel>
            </div>

            <div className="sm:col-span-12">
              <ValidatedLabel className="text-xs font-bold text-slate-700 dark:text-slate-300">
                Description (Optional)
                <ValidatedInput
                  maxLength={500}
                  className={controlClass + " mt-1"}
                  value={draft.description}
                  disabled={!writable || saving}
                  onChange={(e) => markChange({ description: e.target.value })}
                  placeholder="When and where should this checklist format be applied?"
                />
              </ValidatedLabel>
            </div>
          </div>

          {/* Questions Header & Quick Actions */}
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 pb-3 dark:border-slate-800">
            <div className="flex items-center gap-2">
              <h3 className="text-base font-bold text-slate-900 dark:text-white">
                Checklist Questions ({questions.length})
              </h3>
              {duplicates.size > 0 && (
                <span className="rounded-full bg-red-100 px-2.5 py-0.5 text-xs font-bold text-red-700 dark:bg-red-950 dark:text-red-300">
                  {duplicates.size} duplicate wording issues
                </span>
              )}
            </div>

            <div className="flex items-center gap-2">
              {writable && (
                <button
                  type="button"
                  className={secondaryBtn + " gap-1 text-xs py-1 px-3 min-h-9"}
                  onClick={() => setPasteModalOpen(true)}
                  title="Paste multiple questions from Excel/Word"
                >
                  <ClipboardDocumentListIcon className="h-4 w-4 text-blue-600" />
                  Paste Multiple
                </button>
              )}
              <ValidatedInput
                type="search"
                placeholder="Filter questions…"
                className={controlClass + " w-48 text-xs min-h-9"}
                value={questionSearch}
                onChange={(e) => setQuestionSearch(e.target.value)}
              />
            </div>
          </div>

          {/* Section Filter Pills */}
          {sections.length > 1 && (
            <div className="tsc-section-pills">
              <button
                type="button"
                className={`tsc-section-pill ${activeSection === "all" ? "is-active" : ""}`}
                onClick={() => setActiveSection("all")}
              >
                All Sections ({questions.length})
              </button>
              {sections.map((sec) => {
                const count = questions.filter((q) => q.section === sec).length;
                return (
                  <button
                    key={sec}
                    type="button"
                    className={`tsc-section-pill ${activeSection === sec ? "is-active" : ""}`}
                    onClick={() => setActiveSection(sec)}
                  >
                    {sec} ({count})
                  </button>
                );
              })}
            </div>
          )}

          {/* Datalist for Section suggestions */}
          <datalist id="tsc-section-suggestions">
            {sections.map((s) => (
              <option key={s} value={s} />
            ))}
            <option value="Commercial" />
            <option value="Financial" />
            <option value="Statutory" />
            <option value="Scope of Work" />
          </datalist>

          {/* Questions Table with Integrated Bottom Add Row */}
          <div className="tsc-questions-table-wrap">
            <table className="tsc-questions-table">
              <thead>
                <tr>
                  <th style={{ width: "40px" }}>#</th>
                  {writable && <th style={{ width: "50px" }}>Order</th>}
                  <th style={{ width: "160px" }}>Section</th>
                  <th>Question</th>
                  <th style={{ width: "140px" }}>Decision Mode</th>
                  {writable && <th style={{ width: "110px", textAlign: "right" }}>Actions</th>}
                </tr>
              </thead>
              <tbody>
                {visibleQuestions.length === 0 && (
                  <tr>
                    <td
                      colSpan={writable ? 6 : 5}
                      className="py-6 text-center text-xs text-slate-500"
                    >
                      {questions.length
                        ? "No questions match your filter."
                        : "No questions in this checklist format yet. Type below to add your first question."}
                    </td>
                  </tr>
                )}
                {visibleQuestions.map(({ question: q, index }) => {
                  const isDup = duplicates.has(q.id);
                  const isEditingThis = editingQuestionId === q.id;

                  if (isEditingThis) {
                    return (
                      <tr key={q.id} className="bg-blue-50/60 dark:bg-blue-950/40">
                        <td className="font-bold text-slate-500">{index + 1}</td>
                        {writable && <td />}
                        <td>
                          <ValidatedInput
                            list="tsc-section-suggestions"
                            className={controlClass + " text-xs min-h-8 font-semibold"}
                            value={editSection}
                            onChange={(e) => setEditSection(e.target.value)}
                          />
                        </td>
                        <td>
                          <ValidatedTextarea
                            autoFocus
                            rows={2}
                            className={controlClass + " text-xs"}
                            value={editText}
                            onChange={(e) => setEditText(e.target.value)}
                          />
                        </td>
                        <td>
                          <button
                            type="button"
                            className={`tsc-mode-toggle ${editManual ? "is-manual" : "is-ai"}`}
                            onClick={() => setEditManual((m) => !m)}
                          >
                            {editManual ? "Human Review" : "AI Proposal"}
                          </button>
                        </td>
                        <td style={{ textAlign: "right" }}>
                          <div className="flex items-center justify-end gap-1">
                            <button
                              type="button"
                              className="tsc-icon-btn h-8 w-8 text-emerald-700 hover:border-emerald-500"
                              title="Save edits"
                              onClick={() => commitInlineEdit(q.id)}
                            >
                              <CheckIcon className="h-4 w-4" />
                            </button>
                            <button
                              type="button"
                              className="tsc-icon-btn h-8 w-8 text-slate-500"
                              title="Cancel"
                              onClick={() => setEditingQuestionId(null)}
                            >
                              <XMarkIcon className="h-4 w-4" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  }

                  return (
                    <tr
                      key={q.id}
                      className={`tsc-question-row ${isDup ? "is-duplicate" : ""}`}
                    >
                      <td className="font-mono text-xs text-slate-400">
                        {q.printed_number || index + 1}
                      </td>

                      {writable && (
                        <td>
                          <div className="tsc-reorder-group">
                            <button
                              type="button"
                              className="tsc-reorder-btn"
                              title="Move question up"
                              disabled={index === 0}
                              onClick={() => moveQuestion(index, -1)}
                            >
                              <ChevronUpIcon className="h-3 w-3" />
                            </button>
                            <button
                              type="button"
                              className="tsc-reorder-btn"
                              title="Move question down"
                              disabled={index === questions.length - 1}
                              onClick={() => moveQuestion(index, 1)}
                            >
                              <ChevronDownIcon className="h-3 w-3" />
                            </button>
                          </div>
                        </td>
                      )}

                      <td>
                        <span className="inline-block rounded bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                          {q.section}
                        </span>
                      </td>

                      <td>
                        <div
                          className="cursor-pointer py-1 font-medium text-slate-900 transition hover:text-blue-700 dark:text-slate-100 dark:hover:text-blue-400"
                          onClick={() => writable && startInlineEdit(q)}
                          title={writable ? "Click to edit text" : undefined}
                        >
                          {q.question}
                        </div>
                        {isDup && (
                          <p className="mt-1 text-xs font-semibold text-red-600 dark:text-red-400">
                            Duplicate wording of question #{duplicates.get(q.id)}
                          </p>
                        )}
                      </td>

                      <td>
                        <button
                          type="button"
                          className={`tsc-mode-toggle ${q.manual_review ? "is-manual" : "is-ai"}`}
                          disabled={!writable}
                          onClick={() => toggleQuestionManualMode(index)}
                          title={
                            writable
                              ? "Click to toggle AI proposal vs Human decision"
                              : undefined
                          }
                        >
                          {q.manual_review ? (
                            <>
                              <UserIcon className="h-3 w-3" />
                              Human
                            </>
                          ) : (
                            <>
                              <SparklesIcon className="h-3 w-3" />
                              AI Proposal
                            </>
                          )}
                        </button>
                      </td>

                      {writable && (
                        <td style={{ textAlign: "right" }}>
                          <div className="flex items-center justify-end gap-1">
                            <button
                              type="button"
                              className="tsc-icon-btn h-8 w-8"
                              title="Edit question"
                              onClick={() => startInlineEdit(q)}
                            >
                              <PencilSquareIcon className="h-3.5 w-3.5" />
                            </button>
                            <button
                              type="button"
                              className="tsc-icon-btn h-8 w-8"
                              title="Duplicate question"
                              onClick={() => duplicateQuestion(index)}
                            >
                              <DocumentDuplicateIcon className="h-3.5 w-3.5" />
                            </button>
                            <button
                              type="button"
                              className="tsc-icon-btn is-danger h-8 w-8"
                              title="Delete question"
                              onClick={() => removeQuestion(index)}
                            >
                              <TrashIcon className="h-3.5 w-3.5" />
                            </button>
                          </div>
                        </td>
                      )}
                    </tr>
                  );
                })}

                {/* Integrated Add Question Row at the Bottom of Grid */}
                {writable && (
                  <tr className="border-t-2 border-slate-200 bg-slate-50/90 dark:border-slate-700 dark:bg-slate-900/80">
                    <td className="font-mono text-xs font-bold text-blue-600">
                      {questions.length + 1}
                    </td>
                    <td className="text-center text-xs text-slate-400">
                      +
                    </td>
                    <td>
                      <ValidatedInput
                        list="tsc-section-suggestions"
                        className={controlClass + " text-xs min-h-8 font-semibold bg-white dark:bg-slate-950"}
                        value={newSection}
                        onChange={(e) => setNewSection(e.target.value)}
                        placeholder="Section"
                        aria-label="New question section"
                      />
                    </td>
                    <td>
                      <ValidatedInput
                        ref={quickAddInputRef}
                        className={controlClass + " text-xs min-h-8 bg-white dark:bg-slate-950"}
                        value={newQuestionText}
                        onChange={(e) => setNewQuestionText(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            e.preventDefault();
                            handleQuickAddQuestion();
                          }
                        }}
                        placeholder="Type question and press Enter to add…"
                        aria-label="New question text"
                      />
                    </td>
                    <td>
                      <button
                        type="button"
                        className={`tsc-mode-toggle ${newManualReview ? "is-manual" : "is-ai"}`}
                        onClick={() => setNewManualReview((prev) => !prev)}
                        title="Click to toggle evaluation mode"
                      >
                        {newManualReview ? (
                          <>
                            <UserIcon className="h-3 w-3" />
                            Human
                          </>
                        ) : (
                          <>
                            <SparklesIcon className="h-3 w-3" />
                            AI Proposal
                          </>
                        )}
                      </button>
                    </td>
                    <td style={{ textAlign: "right" }}>
                      <div className="flex items-center justify-end gap-1">
                        <button
                          type="button"
                          className={primaryBtn + " gap-1 text-xs py-1 px-3 min-h-8 font-bold"}
                          onClick={() => handleQuickAddQuestion()}
                          disabled={saving || !newQuestionText.trim()}
                          title="Add question to format"
                        >
                          <PlusIcon className="h-3.5 w-3.5" />
                          Add
                        </button>
                        {newQuestionText.trim() ? (
                          <button
                            type="button"
                            className="tsc-icon-btn h-8 w-8 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
                            title="Clear input"
                            onClick={() => setNewQuestionText("")}
                          >
                            <XMarkIcon className="h-4 w-4" />
                          </button>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* Bulk Paste Modal */}
      {pasteModalOpen && (
        <RecordDrawer
          centered
          wide
          title="Paste Multiple Questions"
          onClose={() => setPasteModalOpen(false)}
          footer={
            <div className="flex items-center justify-between gap-3">
              <span className="text-xs text-slate-500">
                {
                  pasteRawText
                    .split(/\r?\n/)
                    .map((l) => l.trim())
                    .filter((l) => l.length > 2).length
                }{" "}
                questions detected
              </span>
              <div className="flex gap-2">
                <button
                  type="button"
                  className={secondaryBtn}
                  onClick={() => setPasteModalOpen(false)}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  className={primaryBtn}
                  disabled={!pasteRawText.trim()}
                  onClick={handleImportPastedQuestions}
                >
                  Import All Questions
                </button>
              </div>
            </div>
          }
        >
          <div className="space-y-4">
            <p className="text-xs text-slate-600 dark:text-slate-400">
              Paste questions below (one question per line). You can include prefixes like{" "}
              <code>Technical: What is the transformer rating?</code> to automatically assign sections.
            </p>

            <ValidatedLabel className="text-xs font-bold text-slate-700 dark:text-slate-300">
              Default Section for Unprefixed Questions
              <ValidatedInput
                list="tsc-section-suggestions"
                className={controlClass + " mt-1"}
                value={pasteDefaultSection}
                onChange={(e) => setPasteDefaultSection(e.target.value)}
              />
            </ValidatedLabel>

            <ValidatedLabel className="text-xs font-bold text-slate-700 dark:text-slate-300">
              Questions Text (one per line)
              <ValidatedTextarea
                rows={10}
                className={controlClass + " mt-1 font-mono text-xs"}
                placeholder={`Is the bidder an Indian registered entity under Companies Act?\nTechnical: Does the manufacturer have minimum 5 years field experience?\nCommercial: What is the payment terms accepted by client?\nFinancial: Has the bidder submitted 3 years audited balance sheets?`}
                value={pasteRawText}
                onChange={(e) => setPasteRawText(e.target.value)}
              />
            </ValidatedLabel>
          </div>
        </RecordDrawer>
      )}

      {/* PDF Document Preview Drawer */}
      {previewFormat && (
        <FormatDocumentPreview
          format={previewFormat}
          onClose={() => setPreviewFormat(null)}
        />
      )}
    </div>
  );
}

function FormatDocumentPreview({
  format,
  onClose,
}: {
  format: TSCFormat;
  onClose: () => void;
}) {
  const [pdf, setPDF] = useState("");
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    let blobURL = "";
    fetch(`${url}/${format.id}/preview.pdf`, {
      cache: "no-store",
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) {
          const body = await response.json().catch(() => ({}));
          throw new Error(body.detail || "Document preview could not be loaded");
        }
        if (!response.headers.get("Content-Type")?.includes("application/pdf")) {
          throw new Error("The service returned an invalid document preview");
        }
        const blob = await response.blob();
        if (controller.signal.aborted) return;
        blobURL = URL.createObjectURL(blob);
        setPDF(blobURL);
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(e.message);
      });
    return () => {
      controller.abort();
      if (blobURL) URL.revokeObjectURL(blobURL);
    };
  }, [format.id, attempt]);

  return (
    <RecordDrawer
      title={`${format.name} — Document Preview`}
      centered
      extraWide
      onClose={onClose}
      footer={
        <div className="flex justify-end gap-2">
          <a
            className={secondaryBtn}
            href={`${url}/${format.id}/template.docx`}
            download
          >
            Download Word Template
          </a>
          <button type="button" className={secondaryBtn} onClick={onClose}>
            Close
          </button>
        </div>
      }
    >
      {error ? (
        <div role="alert" className="p-4 text-center">
          <p className="text-sm font-semibold text-red-600">{error}</p>
          <button
            type="button"
            className={secondaryBtn + " mt-3"}
            onClick={() => {
              setError("");
              setPDF("");
              setAttempt((n) => n + 1);
            }}
          >
            Retry preview
          </button>
        </div>
      ) : pdf ? (
        <div aria-label="Word document preview" role="region">
          <DocumentPDFViewer url={pdf} filename={`${format.name}.pdf`} />
        </div>
      ) : (
        <p role="status" className="p-8 text-center text-sm text-slate-500">
          Preparing document preview…
        </p>
      )}
    </RecordDrawer>
  );
}
