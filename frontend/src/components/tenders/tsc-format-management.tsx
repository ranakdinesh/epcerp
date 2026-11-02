"use client";
import { useEffect, useRef, useState } from "react";
import { PlusIcon, DocumentDuplicateIcon, DocumentTextIcon, ArrowLeftIcon, ArrowRightIcon, EllipsisHorizontalIcon } from "@heroicons/react/24/outline";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { createRecordId } from "@/lib/record-id";
import { duplicateQuestions, questionKey, type TSCFormat, type FormatQuestion } from "./tsc-format-model";

const url = "/api/settings/tsc-formats";
const control = "tsc-format-control";
const action = "ui-action ui-action--secondary";
type Tab = "details" | "questions";
type QuestionPanel = { question: FormatQuestion; index: number; isNew: boolean; opener: HTMLElement | null };
async function request<T>(path = "", init?: RequestInit): Promise<T> {
 const response = await fetch(url + path, { cache: "no-store", ...init });
 const body = await response.json().catch(() => ({ detail: "The format service returned an unreadable response" }));
 if (!response.ok) throw new Error(body.detail || "TSC formats could not be loaded");
 return body;
}

export function TSCFormatManagement() {
 const user = useAuthenticatedUser();
 const writable = user.permissions.includes("system:admin") || user.permissions.includes("settings.tsc_formats.write");
 const [formats, setFormats] = useState<TSCFormat[]>([]), [draft, setDraft] = useState<TSCFormat | null>(null);
 const [search, setSearch] = useState(""), [filter, setFilter] = useState("active");
 const [questionSearch, setQuestionSearch] = useState(""), [tab, setTab] = useState<Tab>("questions");
 const [panel, setPanel] = useState<QuestionPanel | null>(null);
 const [loading, setLoading] = useState(true), [saving, setSaving] = useState(false), [dirty, setDirty] = useState(false);
 const [error, setError] = useState(""), [notice, setNotice] = useState("");
 const [removed, setRemoved] = useState<{ question: FormatQuestion; index: number } | null>(null);
 const errorRef = useRef<HTMLDivElement>(null), headingRef = useRef<HTMLHeadingElement>(null);
 const librarySearch = useRef<HTMLInputElement>(null), selection = useRef(0), busy = useRef(false);
 const detailsTab = useRef<HTMLButtonElement>(null), questionsTab = useRef<HTMLButtonElement>(null);
 useEffect(() => {
  const c = new AbortController();
  request<{ items: TSCFormat[] }>("", { signal: c.signal }).then(body => setFormats(body.items))
   .catch(e => { if (!c.signal.aborted) setError(e.message); })
   .finally(() => { if (!c.signal.aborted) setLoading(false); });
  return () => c.abort();
 }, []);
 useEffect(() => {
  if (!dirty) return;
  const warn = (e: BeforeUnloadEvent) => { e.preventDefault(); };
  window.addEventListener("beforeunload", warn); return () => window.removeEventListener("beforeunload", warn);
 }, [dirty]);
 const questions = draft?.questions || [];
 const duplicates = duplicateQuestions(questions);
 const shown = formats.filter(f => (filter === "all" || f.is_active === (filter === "active")) && `${f.name} ${f.description}`.toLowerCase().includes(search.toLowerCase()));
 const visibleQuestions = questions.map((question, index) => ({ question, index })).filter(({ question }) => `${question.question} ${question.section}`.toLowerCase().includes(questionSearch.toLowerCase()));
 function canLeave() { return !dirty || window.confirm("Discard unsaved changes to this TSC format?"); }
 function focusHeading() { requestAnimationFrame(() => { headingRef.current?.focus(); window.scrollTo(0, 0); }); }
 function change(patch: Partial<TSCFormat>) { setDraft(previous => previous ? { ...previous, ...patch } : previous); setDirty(true); setNotice(""); }
 async function open(id: string) {
  if (saving || !canLeave()) return;
  const current = ++selection.current; setLoading(true); setError(""); setNotice("");
  try {
   const value = await request<TSCFormat>(`/${id}`); if (selection.current !== current) return;
   setDraft(value); setDirty(false); setRemoved(null); setTab("questions"); setQuestionSearch(""); focusHeading();
  } catch (e) { if (selection.current === current) setError(e instanceof Error ? e.message : "Could not open format"); }
  finally { if (selection.current === current) setLoading(false); }
 }
 function create(copy = false) {
  if (saving || !canLeave()) return;
  selection.current++; setLoading(false);
  setDraft({ id: "", name: copy && draft ? `${draft.name} — copy` : "", description: copy && draft ? draft.description : "", is_active: true, version: 0, question_count: copy ? questions.length : 0, questions: copy ? questions.map(q => ({ ...q })) : [] });
  setDirty(copy); setError(""); setNotice(""); setRemoved(null); setTab("details"); setQuestionSearch(""); focusHeading();
 }
 function back() {
  if (saving || !canLeave()) return;
  selection.current++; setDraft(null); setError(""); setNotice(""); setDirty(false); setRemoved(null); setLoading(false);
  requestAnimationFrame(() => { librarySearch.current?.focus(); window.scrollTo(0, 0); });
 }
 function changeTab(next: Tab, focus = false) {
  setTab(next);
  if (focus) (next === "details" ? detailsTab : questionsTab).current?.focus();
 }
 function tabKey(event: React.KeyboardEvent) {
  if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
  event.preventDefault(); changeTab(event.key === "Home" ? "details" : event.key === "End" ? "questions" : tab === "details" ? "questions" : "details", true);
 }
 function editQuestion(index: number) { setPanel({ question: { ...questions[index] }, index, isNew: false, opener: document.activeElement as HTMLElement }); }
 function addQuestion() { setPanel({ question: { id: createRecordId(), section: questions.at(-1)?.section || "General", printed_number: "", question: "", manual_review: false }, index: questions.length, isNew: true, opener: document.activeElement as HTMLElement }); }
 function move(index: number, delta: number) {
  const next = [...questions]; [next[index], next[index + delta]] = [next[index + delta], next[index]]; change({ questions: next });
 }
 function remove(index: number) { setRemoved({ question: questions[index], index }); change({ questions: questions.filter((_, i) => i !== index) }); }
 function undo() { if (!removed) return; const next = [...questions]; next.splice(removed.index, 0, removed.question); change({ questions: next }); setRemoved(null); }
 async function save() {
  if (!draft || busy.current || !writable) return;
  if (draft.name.trim().length < 3) { setTab("details"); setError("Give this format a name of at least three characters."); requestAnimationFrame(() => errorRef.current?.focus()); return; }
  if (!questions.length || duplicates.size) { setTab("questions"); setError(duplicates.size ? "Remove or rewrite duplicate questions before saving." : "Add at least one question before saving this format."); requestAnimationFrame(() => errorRef.current?.focus()); return; }
  busy.current = true; setSaving(true); setError(""); setNotice("");
  try {
   const result = await request<TSCFormat>(draft.id ? `/${draft.id}` : "", { method: draft.id ? "PATCH" : "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name: draft.name, description: draft.description, is_active: draft.is_active, expected_version: draft.version, questions }) });
   setDraft(result); setFormats(previous => [...previous.filter(f => f.id !== result.id), result].sort((a, b) => a.name.localeCompare(b.name)));
   setDirty(false); setRemoved(null); setNotice("Format saved. New checklists will use this revision.");
  } catch (e) { setError(e instanceof Error ? e.message : "Format could not be saved"); requestAnimationFrame(() => errorRef.current?.focus()); }
  finally { busy.current = false; setSaving(false); }
 }
 return <section className="tsc-formats" aria-label="TSC format management">
  {draft ? <div className="tsc-format-workspace-bar"><button type="button" className="tsc-format-back" disabled={saving} onClick={back}><ArrowLeftIcon aria-hidden="true" />All TSC formats</button><div className="tsc-format-actions">{writable ? <>{draft.id ? <details className="tsc-format-question-menu"><summary aria-label="Format actions"><EllipsisHorizontalIcon aria-hidden="true" /></summary><div><button type="button" disabled={saving} onClick={() => create(true)}><DocumentDuplicateIcon aria-hidden="true" />Make a copy</button></div></details> : null}<button type="button" className="ui-action ui-action--primary" disabled={saving || !questions.length || (!!draft.id && !dirty)} onClick={() => void save()}>{saving ? "Saving…" : "Save format"}</button></> : <span className="tsc-format-status">Read only</span>}</div></div> : null}
  <header className="tsc-formats-heading">
   <div><p className="tsc-format-eyebrow">Global masters{draft ? " / TSC formats" : ""}</p><h1 ref={headingRef} tabIndex={-1}>{draft ? draft.name || "New TSC format" : "TSC formats"}</h1>{draft ? <p className="tsc-format-subtitle">{`${questions.length} questions · ${draft.id ? `Revision ${draft.version}` : "Not saved yet"}${dirty ? " · Unsaved changes" : ""}`}</p> : null}</div>
   {!draft && writable ? <button type="button" className="ui-action ui-action--primary" onClick={() => create()} disabled={loading}><PlusIcon aria-hidden="true" />New format</button> : null}
  </header>
  {notice ? <p role="status" className="tsc-format-notice">{notice}</p> : null}
  {error ? <div ref={errorRef} tabIndex={-1} role="alert" className="tsc-format-error"><strong>Could not complete this change</strong><p>{error}</p>{draft?.id ? <button type="button" className={action} onClick={() => void open(draft.id)}>Reload saved format</button> : !draft ? <button type="button" className={action} onClick={() => window.location.reload()}>Reload formats</button> : null}</div> : null}
  {loading ? <p role="status" className="tsc-format-loading">Loading formats…</p> : !draft ? <>
   <div className="tsc-format-library-filters"><div><ValidatedLabel htmlFor="format-search">Find a format</ValidatedLabel><ValidatedInput ref={librarySearch} id="format-search" type="search" placeholder="Search formats" className={control} value={search} onChange={e => setSearch(e.target.value)} /></div><div><ValidatedLabel htmlFor="format-status">Show formats</ValidatedLabel><ValidatedSelect id="format-status" className={control} value={filter} onChange={e => setFilter(e.target.value)}><option value="active">Active formats</option><option value="inactive">Inactive formats</option><option value="all">All formats</option></ValidatedSelect></div></div>
   <div className="tsc-format-library" role="list" aria-label="Saved TSC formats">{shown.map(f => <article key={f.id} role="listitem"><button type="button" className="tsc-format-card" aria-label={`Open ${f.name}`} onClick={() => void open(f.id)}><span className="tsc-format-card-top"><span className="tsc-format-document-icon"><DocumentTextIcon aria-hidden="true" /></span><ArrowRightIcon aria-hidden="true" /></span><span className="tsc-format-card-name">{f.name === "EPC Works — Standard TSC" ? "EPC Works" : f.name}</span><span className="tsc-format-meta">{f.question_count} questions{!f.is_active ? " · Inactive" : ""}</span></button></article>)}</div>
   {!shown.length ? <div className="tsc-format-empty"><DocumentTextIcon aria-hidden="true" /><h2>No formats to show</h2><p>{formats.length ? "Try another search or availability filter." : "Create your first checklist format to get started."}</p></div> : null}
  </> : <>
   <div className="tsc-format-tabs" role="tablist" aria-label="Format workspace"><button ref={detailsTab} type="button" role="tab" id="format-details-tab" aria-controls="format-details-panel" aria-selected={tab === "details"} tabIndex={tab === "details" ? 0 : -1} onKeyDown={tabKey} onClick={() => changeTab("details")}>Format details</button><button ref={questionsTab} type="button" role="tab" id="format-questions-tab" aria-controls="format-questions-panel" aria-selected={tab === "questions"} tabIndex={tab === "questions" ? 0 : -1} onKeyDown={tabKey} onClick={() => changeTab("questions")}>Questions <span>{questions.length}</span></button></div>
   {tab === "details" ? <section role="tabpanel" id="format-details-panel" aria-labelledby="format-details-tab" className="tsc-format-details-view"><div className="tsc-format-section-intro"><h2>About this format</h2></div><ValidatedForm aria-label="TSC format details" onSubmit={e => { e.preventDefault(); changeTab("questions", true); }}><fieldset disabled={!writable || saving} className="tsc-format-details-fields"><div><ValidatedLabel htmlFor="format-name">Format name</ValidatedLabel><ValidatedInput id="format-name" required minLength={3} maxLength={120} className={control} placeholder="e.g. EPC Works — Standard TSC" value={draft.name} onChange={e => change({ name: e.target.value })} /></div><div><ValidatedLabel htmlFor="format-description">Description (optional)</ValidatedLabel><ValidatedTextarea id="format-description" rows={5} maxLength={1000} className={control} placeholder="When should this format be used?" value={draft.description} onChange={e => change({ description: e.target.value })} /></div><div className="tsc-format-availability"><ValidatedLabel htmlFor="format-active">Availability</ValidatedLabel><ValidatedSelect id="format-active" className={control} value={String(draft.is_active)} onChange={e => change({ is_active: e.target.value === "true" })}><option value="true">Active · available for tenders</option><option value="false">Inactive · retained for history</option></ValidatedSelect><p>Existing checklists keep their original questions when you change or deactivate a format.</p></div></fieldset><button type="submit" className={action}>Continue to questions<ArrowRightIcon aria-hidden="true" /></button></ValidatedForm></section> : <section role="tabpanel" id="format-questions-panel" aria-labelledby="format-questions-tab" className="tsc-format-questions-view">
    <div className="tsc-format-section-intro tsc-format-question-toolbar"><div><h2>Checklist questions</h2></div>{writable ? <button type="button" className="ui-action ui-action--primary" disabled={saving || questions.length >= 200} onClick={addQuestion}><PlusIcon aria-hidden="true" />Add question</button> : null}</div>
    {removed ? <div role="status" className="tsc-format-notice">Question removed from this draft. <button type="button" className={action} disabled={saving} onClick={undo}>Undo removal</button></div> : null}
    {questions.length ? <><div className="tsc-format-question-search"><ValidatedLabel htmlFor="format-question-search">Find a question</ValidatedLabel><ValidatedInput id="format-question-search" type="search" className={control} placeholder="Search question text or section" value={questionSearch} onChange={e => setQuestionSearch(e.target.value)} /></div><ol className="tsc-format-question-list" aria-label="Format questions">{visibleQuestions.map(({ question: q, index }) => <li key={q.id} className="tsc-format-question-row"><span className="tsc-format-question-position" aria-hidden="true">{index + 1}</span><button type="button" className="tsc-format-question-open" aria-label={`${writable ? "Edit" : "View"} question ${index + 1}`} aria-describedby={`question-summary-${q.id}`} disabled={saving} onClick={() => editQuestion(index)}><span>{q.section.replaceAll("_", " ")}{q.manual_review ? " · Human decision" : ""}</span><strong id={`question-summary-${q.id}`}>{q.question}</strong></button><ArrowRightIcon aria-hidden="true" className="tsc-format-question-chevron" />{writable ? <details className="tsc-format-question-menu"><summary aria-label={`Actions for question ${index + 1}`}><EllipsisHorizontalIcon aria-hidden="true" /></summary><div><button type="button" disabled={saving || index === 0} onClick={e => { e.currentTarget.closest("details")?.removeAttribute("open"); move(index, -1); }}>Move up</button><button type="button" disabled={saving || index === questions.length - 1} onClick={e => { e.currentTarget.closest("details")?.removeAttribute("open"); move(index, 1); }}>Move down</button><button type="button" className="tsc-format-remove" disabled={saving} onClick={() => remove(index)}>Remove question</button></div></details> : null}</li>)}</ol>{!visibleQuestions.length ? <p className="tsc-format-empty">No questions match your search.</p> : null}</> : <div className="tsc-format-empty"><DocumentTextIcon aria-hidden="true" /><h3>Start with your first question</h3><p>Add questions one at a time. Each opens in a separate editor with room to write.</p>{writable ? <button type="button" className={action} onClick={addQuestion}>Add the first question</button> : null}</div>}
   </section>}
  </>}
  {panel ? <QuestionEditor key={panel.question.id} panel={panel} questions={questions} writable={writable} close={() => setPanel(null)} apply={value => { change({ questions: panel.isNew ? [...questions, value] : questions.map(q => q.id === value.id ? value : q) }); setPanel(null); }} /> : null}
 </section>;
}

function QuestionEditor({ panel, questions, writable, close, apply }: { panel: QuestionPanel; questions: FormatQuestion[]; writable: boolean; close: () => void; apply: (value: FormatQuestion) => void }) {
 const [value, setValue] = useState(panel.question);
 const dirty = JSON.stringify(value) !== JSON.stringify(panel.question);
 const duplicateIndex = questions.findIndex(q => q.id !== value.id && questionKey(q.question) === questionKey(value.question) && questionKey(value.question) !== "");
 useEffect(() => {
  if (!dirty) return; const warn = (e: BeforeUnloadEvent) => e.preventDefault();
  window.addEventListener("beforeunload", warn); return () => window.removeEventListener("beforeunload", warn);
 }, [dirty]);
 function dismiss() { if (!dirty || window.confirm("Discard edits to this question?")) close(); }
 return <RecordDrawer centered wide title={panel.isNew ? "Add a question" : `${writable ? "Edit" : "View"} question ${panel.index + 1}`} onClose={dismiss} returnFocusTo={panel.opener} footer={<div className="tsc-question-editor-footer"><p>{writable ? "Apply to this draft, then save the format when you are ready." : "This format is available for viewing."}</p><div className="tsc-format-actions"><button type="button" className={action} onClick={dismiss}>{writable ? "Cancel" : "Close"}</button>{writable ? <button type="submit" form="tsc-question-editor-form" className="ui-action ui-action--primary" disabled={duplicateIndex >= 0}>Apply to format</button> : null}</div></div>}>
  <ValidatedForm id="tsc-question-editor-form" aria-label="Question editor" onSubmit={e => { e.preventDefault(); if (writable && duplicateIndex < 0) apply(value); }} className="tsc-question-editor-form">
   <p className="tsc-question-editor-intro">Write a clear, specific question. Keep one subject per question.</p>
   <fieldset disabled={!writable}><div><ValidatedLabel htmlFor="question-editor-text">Question text</ValidatedLabel><ValidatedTextarea id="question-editor-text" autoFocus required minLength={3} maxLength={2000} rows={7} className={control} value={value.question} error={duplicateIndex >= 0 ? `This question already exists as question ${duplicateIndex + 1}. Change the wording before applying it.` : undefined} onChange={e => setValue(previous => ({ ...previous, question: e.target.value }))} /></div><div><ValidatedLabel htmlFor="question-editor-section">Section</ValidatedLabel><ValidatedInput id="question-editor-section" required maxLength={80} className={control} value={value.section} onChange={e => setValue(previous => ({ ...previous, section: e.target.value }))} /></div><details className="tsc-question-editor-options"><summary>Numbering and answer preparation</summary><div><ValidatedLabel htmlFor="question-editor-number">Display number (optional)</ValidatedLabel><ValidatedInput id="question-editor-number" maxLength={30} placeholder={String(panel.index + 1)} className={control} value={value.printed_number} onChange={e => setValue(previous => ({ ...previous, printed_number: e.target.value }))} /></div><div><ValidatedLabel htmlFor="question-editor-review">Answer preparation</ValidatedLabel><ValidatedSelect id="question-editor-review" className={control} value={value.manual_review ? "manual" : "source"} onChange={e => setValue(previous => ({ ...previous, manual_review: e.target.value === "manual" }))}><option value="source">Source documents / AI proposal</option><option value="manual">Human decision · no AI proposal</option></ValidatedSelect></div></details></fieldset>
  </ValidatedForm>
 </RecordDrawer>;
}
