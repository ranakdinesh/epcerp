"use client";
import { useEffect, useRef, useState } from "react";
import { PlusIcon, DocumentDuplicateIcon, ArrowUpIcon, ArrowDownIcon, TrashIcon, DocumentTextIcon } from "@heroicons/react/24/outline";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { createRecordId } from "@/lib/record-id";
import { duplicateQuestions, type TSCFormat, type FormatQuestion } from "./tsc-format-model";

const url = "/api/settings/tsc-formats";
const control = "min-h-11 w-full rounded-lg border border-slate-300 bg-white p-2 text-sm dark:border-slate-600 dark:bg-slate-900";
const action = "ui-action ui-action--secondary";
const question = (section = "General"): FormatQuestion => ({ id: createRecordId(), section, printed_number: "", question: "", manual_review: false });
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
 const [loading, setLoading] = useState(true), [saving, setSaving] = useState(false), [dirty, setDirty] = useState(false);
 const [error, setError] = useState(""), [notice, setNotice] = useState("");
 const [removed, setRemoved] = useState<{ question: FormatQuestion; index: number } | null>(null);
 const errorRef = useRef<HTMLDivElement>(null), selection = useRef(0), busy = useRef(false);
 useEffect(() => { const c = new AbortController(); request<{ items: TSCFormat[] }>("", { signal: c.signal }).then(body => setFormats(body.items)).catch(e => { if (!c.signal.aborted) setError(e.message); }).finally(() => { if (!c.signal.aborted) setLoading(false); }); return () => c.abort(); }, []);
 useEffect(() => { if (!dirty) return; const warn = (e: BeforeUnloadEvent) => { e.preventDefault(); }; window.addEventListener("beforeunload", warn); return () => window.removeEventListener("beforeunload", warn); }, [dirty]);
 const questions = draft?.questions || [];
 const duplicates = duplicateQuestions(questions);
 const shown = formats.filter(f => (filter === "all" || f.is_active === (filter === "active")) && `${f.name} ${f.description}`.toLowerCase().includes(search.toLowerCase()));
 function canLeave() { return !dirty || window.confirm("Discard unsaved changes to this TSC format?"); }
 function change(patch: Partial<TSCFormat>) { setDraft(previous => previous ? { ...previous, ...patch } : previous); setDirty(true); setNotice(""); }
 async function open(id: string) {
  if (saving || !canLeave()) return;
  const current = ++selection.current; setLoading(true); setError(""); setNotice("");
  try { const value = await request<TSCFormat>(`/${id}`); if (selection.current !== current) return; setDraft(value); setDirty(false); setRemoved(null); }
  catch (e) { if (selection.current === current) setError(e instanceof Error ? e.message : "Could not open format"); }
  finally { if (selection.current === current) setLoading(false); }
 }
 function create(copy = false) {
  if (saving || !canLeave()) return;
  selection.current++; setLoading(false);
  setDraft({ id: "", name: copy && draft ? `${draft.name} — copy` : "", description: copy && draft ? draft.description : "", is_active: true, version: 0, question_count: copy ? questions.length : 1, questions: copy ? questions.map(q => ({ ...q })) : [question()] });
  setDirty(copy); setError(""); setNotice(""); setRemoved(null);
 }
 function updateQuestion(id: string, patch: Partial<FormatQuestion>) { change({ questions: questions.map(q => q.id === id ? { ...q, ...patch } : q) }); }
 function move(index: number, delta: number) { const next = [...questions]; [next[index], next[index + delta]] = [next[index + delta], next[index]]; change({ questions: next }); }
 function remove(index: number) { setRemoved({ question: questions[index], index }); change({ questions: questions.filter((_, i) => i !== index) }); }
 function undo() { if (!removed) return; const next = [...questions]; next.splice(removed.index, 0, removed.question); change({ questions: next }); setRemoved(null); }
 async function save() {
  if (!draft || busy.current || !writable) return;
  if (duplicates.size) { setError("Remove or rewrite duplicate questions before saving."); requestAnimationFrame(() => errorRef.current?.focus()); return; }
  busy.current = true; setSaving(true); setError(""); setNotice("");
  try {
   const result = await request<TSCFormat>(draft.id ? `/${draft.id}` : "", { method: draft.id ? "PATCH" : "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name: draft.name, description: draft.description, is_active: draft.is_active, expected_version: draft.version, questions }) });
   setDraft(result); setFormats(previous => [...previous.filter(f => f.id !== result.id), result].sort((a, b) => a.name.localeCompare(b.name))); setDirty(false); setRemoved(null); setNotice("TSC format saved. Existing generated checklists keep their original questions.");
  } catch (e) { setError(e instanceof Error ? e.message : "Format could not be saved"); requestAnimationFrame(() => errorRef.current?.focus()); }
  finally { busy.current = false; setSaving(false); }
 }
 return <section className="tsc-formats" aria-label="TSC format management">
  <div className="go-page-heading tsc-formats-heading"><div><p className="go-eyebrow">Global masters</p><h1>TSC formats</h1><p>Build the right checklist for every kind of tender.</p></div>{writable ? <button type="button" className="ui-action ui-action--primary" onClick={() => create()} disabled={saving}><PlusIcon aria-hidden="true" />New format</button> : null}</div>
  {notice ? <p role="status" className="tsc-review-notice">{notice}</p> : null}
  <div className="tsc-format-layout">
   <aside className="tsc-format-library" aria-label="Saved TSC formats"><header><h2>Format library <span>{formats.length}</span></h2><ValidatedLabel htmlFor="format-search">Search formats</ValidatedLabel><ValidatedInput id="format-search" type="search" className={control} value={search} onChange={e => setSearch(e.target.value)} /><ValidatedLabel htmlFor="format-status">Show formats</ValidatedLabel><ValidatedSelect id="format-status" className={control} value={filter} onChange={e => setFilter(e.target.value)}><option value="active">Active formats</option><option value="inactive">Inactive formats</option><option value="all">All formats</option></ValidatedSelect></header>
    <div className="tsc-format-library-list">{shown.map(f => <button type="button" key={f.id} className="tsc-format-choice" aria-pressed={draft?.id === f.id} onClick={() => void open(f.id)} disabled={loading || saving}><DocumentTextIcon aria-hidden="true" /><span><strong>{f.name}</strong><small>{f.question_count} questions · Revision {f.version}{f.is_active ? "" : " · Inactive"}</small></span></button>)}{!shown.length ? <p className="tsc-muted">{loading ? "Loading formats…" : "No formats match these filters."}</p> : null}</div>
   </aside>
   <div className="tsc-format-editor">
    {error ? <div ref={errorRef} tabIndex={-1} role="alert" className="tsc-format-error"><strong>Check this format</strong><p>{error}</p>{duplicates.size ? <ul>{questions.flatMap((q, i) => duplicates.has(q.id) ? [<li key={q.id}><a href={`#format-question-${q.id}`}>Question {i + 1} duplicates question {duplicates.get(q.id)}</a></li>] : [])}</ul> : null}{draft?.id ? <button type="button" className={action} onClick={() => void open(draft.id)}>Reload saved format</button> : <button type="button" className={action} onClick={() => window.location.reload()}>Reload formats</button>}</div> : null}
    {loading ? <p role="status">Loading format…</p> : draft ? <ValidatedForm aria-label="TSC format editor" onSubmit={e => { e.preventDefault(); void save(); }}>
     <div className="tsc-format-editor-bar"><div><p className="tsc-section-label">{draft.id ? `Revision ${draft.version}` : "New format"}</p><h2>{draft.name || "Untitled TSC format"}</h2><p className="tsc-muted">{questions.length} questions · {dirty ? "Unsaved changes" : draft.id ? "Up to date" : "Not saved yet"}</p></div><div className="tsc-format-actions">{writable ? <>{draft.id ? <button type="button" className={action} disabled={saving} onClick={() => create(true)}><DocumentDuplicateIcon aria-hidden="true" />Make a copy</button> : null}<button type="submit" className="ui-action ui-action--primary" disabled={saving || duplicates.size > 0 || (!!draft.id && !dirty)}>{saving ? "Saving…" : "Save format"}</button></> : <span className="tsc-answer-badge">Read only</span>}</div></div>
     <fieldset disabled={!writable || saving} className="tsc-format-fields"><div><ValidatedLabel htmlFor="format-name">Format name</ValidatedLabel><ValidatedInput id="format-name" required minLength={3} maxLength={120} className={control} placeholder="e.g. EPC Works — Standard TSC" value={draft.name} onChange={e => change({ name: e.target.value })} /></div><div><ValidatedLabel htmlFor="format-active">Availability</ValidatedLabel><ValidatedSelect id="format-active" className={control} value={String(draft.is_active)} onChange={e => change({ is_active: e.target.value === "true" })}><option value="true">Active · available for tenders</option><option value="false">Inactive · retained for history</option></ValidatedSelect></div><div className="tsc-format-description"><ValidatedLabel htmlFor="format-description">Description (optional)</ValidatedLabel><ValidatedTextarea id="format-description" rows={2} maxLength={1000} className={control} value={draft.description} onChange={e => change({ description: e.target.value })} /></div></fieldset>
     <div className="tsc-format-question-bar"><div><h3>Checklist questions</h3><p className="tsc-muted">Order questions as they should appear. Duplicate wording is blocked within this format.</p></div>{writable ? <button type="button" className={action} disabled={saving || questions.length >= 200} onClick={() => change({ questions: [...questions, question(questions.at(-1)?.section)] })}><PlusIcon aria-hidden="true" />Add question</button> : null}</div>
     {removed ? <div role="status" className="tsc-review-notice">Question removed from this draft. <button type="button" className={action} disabled={saving} onClick={undo}>Undo removal</button></div> : null}
     <fieldset disabled={!writable || saving} className="tsc-format-questions">{questions.map((q, index) => <section key={q.id} className="tsc-format-question" aria-label={`Format question ${index + 1}`} data-duplicate={duplicates.has(q.id)}><div className="tsc-format-question-top"><h4><span>{index + 1}</span>Question</h4>{writable ? <div className="tsc-format-actions"><button type="button" className={`${action} ui-action--icon`} aria-label={`Move question ${index + 1} up`} disabled={index === 0} onClick={() => move(index, -1)}><ArrowUpIcon aria-hidden="true" /></button><button type="button" className={`${action} ui-action--icon`} aria-label={`Move question ${index + 1} down`} disabled={index === questions.length - 1} onClick={() => move(index, 1)}><ArrowDownIcon aria-hidden="true" /></button><button type="button" className="ui-action ui-action--danger ui-action--icon" aria-label={`Remove question ${index + 1}`} disabled={questions.length === 1} onClick={() => remove(index)}><TrashIcon aria-hidden="true" /></button></div> : null}</div>
      <ValidatedLabel htmlFor={`format-question-${q.id}`}>Question text</ValidatedLabel><ValidatedTextarea id={`format-question-${q.id}`} required minLength={3} maxLength={2000} rows={2} className={control} value={q.question} error={duplicates.has(q.id) ? `Duplicates question ${duplicates.get(q.id)}. Edit the wording or remove this question.` : undefined} onChange={e => updateQuestion(q.id, { question: e.target.value })} />
      <div className="tsc-format-question-meta"><div><ValidatedLabel htmlFor={`section-${q.id}`}>Section</ValidatedLabel><ValidatedInput id={`section-${q.id}`} required maxLength={80} className={control} value={q.section} onChange={e => updateQuestion(q.id, { section: e.target.value })} /></div><div><ValidatedLabel htmlFor={`number-${q.id}`}>Display number (optional)</ValidatedLabel><ValidatedInput id={`number-${q.id}`} maxLength={30} className={control} placeholder={String(index + 1)} value={q.printed_number} onChange={e => updateQuestion(q.id, { printed_number: e.target.value })} /></div><div><ValidatedLabel htmlFor={`review-${q.id}`}>Answer preparation</ValidatedLabel><ValidatedSelect id={`review-${q.id}`} className={control} value={q.manual_review ? "manual" : "source"} onChange={e => updateQuestion(q.id, { manual_review: e.target.value === "manual" })}><option value="source">Source documents / AI proposal</option><option value="manual">Human decision · no AI proposal</option></ValidatedSelect></div></div>
     </section>)}</fieldset>
    </ValidatedForm> : <div className="tsc-format-empty"><DocumentTextIcon aria-hidden="true" /><h2>A checklist for every tender</h2><p>Select a saved format to review its questions, or create a format for a different tender type.</p><p className="tsc-muted">Changes apply to new generations. Existing TSC drafts keep their format revision and answers.</p>{writable ? <button type="button" className="ui-action ui-action--primary" onClick={() => create()}>Create a TSC format</button> : null}</div>}
   </div>
  </div>
 </section>;
}
