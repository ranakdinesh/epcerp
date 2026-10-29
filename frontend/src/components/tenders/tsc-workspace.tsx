"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { PlusIcon } from "@heroicons/react/24/outline";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { editableTSCAnswer, placeholderTSCText } from "./tsc-presentation";
import { TSCResearchPanel, providerLabel, type ProviderOptions } from "./tsc-research-panel";
type Question = { id: string; section: string; printed_number: string; question: string };
type Passage = { id: string; document_id: string; filename: string; locator: string; text: string; heading?: string; kind?: string; method?: string; suppressed?: boolean; confidence?: number };
type Coverage = { version: string; documents: number; segments: number; empty_segments: number; blocks: number; suppressed: number; low_confidence: number };
type Diagnostic = { question_id: string; candidates: number; documents: number; outcome: string };
type Citation = { source_id: string; quote: string };
type Answer = { accepted?: boolean; question_id: string; status: string; answer: string; citations: Citation[]; origin: string };
type Job = { id: string; state: string; error: string; version: number; draft_version: number; mode: string; model: string; attempts: number; stale: boolean; created_at: string; input: { questions: Question[]; passages: Passage[]; coverage?: Coverage; proposal_version?: string }; result: { answers: Answer[]; warnings: string[]; diagnostics?: Diagnostic[] } };
const primary = "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white hover:bg-blue-800 disabled:opacity-50";
const secondary = "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-slate-300 px-3 text-sm font-semibold hover:bg-slate-100 disabled:opacity-50";
const control = "min-h-11 w-full rounded-xl border border-slate-300 bg-white p-2 text-sm text-slate-900 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100";
const message = (e: unknown) => e instanceof Error ? e.message : "TSC request could not complete";
async function call<T>(path: string, init?: RequestInit): Promise<T> {
 const response = await fetch(`/api/tender-fms/${path}`, { cache: "no-store", ...init });
 const body = await response.json().catch(() => ({ detail: "TSC service returned an unreadable response" }));
 if (!response.ok) throw new Error(body.detail || "TSC request could not complete");
 return body as T;
}
export function TSCWorkspace({ draftID, sourceVersion, readyForGeneration = true }: { draftID: string; sourceVersion: number; readyForGeneration?: boolean }) {
 const user = useAuthenticatedUser();
 const has = (permission: string) => user.permissions.includes("system:admin") || user.permissions.includes(permission);
 const canRead = has("tenders.tsc.read"), canWrite = has("tenders.tsc.write") && has("tenders.register.write") && has("tenders.processing.write"), canExport = has("tenders.tsc.export");
 const [jobs, setJobs] = useState<Job[]>([]), [selectedID, setSelectedID] = useState("");
 const [job, setJob] = useState<Job | null>(null), [error, setError] = useState("");
 const [busy, setBusy] = useState(false), [mode, setMode] = useState("");
 const [options, setOptions] = useState<ProviderOptions>({ providers: [], limit_paise: 10000, reserved_paise: 0 }), [optionsError, setOptionsError] = useState("");
 const base = `drafts/${draftID}/tsc`;
 const operation = useRef<{ fingerprint: string; key: string } | null>(null);
 const loadOptions = useCallback(async () => { try { setOptions(await call<ProviderOptions>(`${base}/options`)); setOptionsError(""); } catch (e) { setOptionsError(message(e)); } }, [base]);
 const load = useCallback(async (signal?: AbortSignal) => { try { const result = await call<{ items: Job[] }>(base, { signal }); setJobs(result.items); setSelectedID(previous => previous || result.items[0]?.id || ""); setError(""); } catch (e) { if (!signal?.aborted) setError(message(e)); } }, [base]);
 useEffect(() => { if (!canRead) return; const c = new AbortController(); const timer = setTimeout(() => { void load(c.signal); void loadOptions(); }, 0); return () => { clearTimeout(timer); c.abort(); }; }, [canRead, load, loadOptions, sourceVersion]);
 useEffect(() => {
  if (!canRead || !selectedID) return;
  const c = new AbortController(); let timer: ReturnType<typeof setTimeout>; let alive = true;
  async function refresh() { try { const result = await call<Job>(`${base}/${selectedID}`, { signal: c.signal }); if (!alive) return; setJob(result); if (result.state === "queued" || result.state === "processing") timer = setTimeout(() => void refresh(), 3000); else void loadOptions(); } catch (e) { if (alive) setError(message(e)); } }
  timer = setTimeout(() => void refresh(), 0); return () => { alive = false; clearTimeout(timer); c.abort(); };
 }, [base, selectedID, canRead, sourceVersion, loadOptions]);
 if (!canRead) return <p>TSC draft access requires the TSC read permission.</p>;
 const selectedMode = mode || options.providers.find(p => p.available && p.mode === "gemini")?.mode || options.providers.find(p => p.available && p.mode === "ollama")?.mode || "manual";
 const pending = job?.state === "queued" || job?.state === "processing";
 const stale = job?.stale || (job && job.draft_version !== sourceVersion);
 async function generate() {
  setBusy(true); setError("");
  try { const fingerprint = `${draftID}:${selectedMode}:${sourceVersion}`; if (operation.current?.fingerprint !== fingerprint) operation.current = { fingerprint, key: crypto.randomUUID() };
   const result = await call<Job>(base, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": operation.current.key }, body: JSON.stringify({ mode: selectedMode, expected_version: sourceVersion }) });
   operation.current = null; setJob(result); setSelectedID(result.id); await load(); await loadOptions();
  } catch (e) { setError(message(e)); } finally { setBusy(false); }
 }
 return <section className="space-y-4 border-t border-slate-200 pt-5 text-slate-900 dark:text-slate-100" aria-label="TSC draft workspace">
  <header className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between"><div><h2 className="text-lg font-bold">Tender Selection Checklist</h2><p className="text-sm text-slate-600">Edit the TSC on the left while reading the consolidated source Markdown on the right.</p></div>
  {canWrite ? <div className="flex flex-col gap-2"><ValidatedLabel htmlFor="tsc-mode">Draft preparation</ValidatedLabel><ValidatedSelect id="tsc-mode" className={control} value={selectedMode} onChange={e => setMode(e.target.value)}>{(options.providers.length ? options.providers : [{ mode: "manual", available: true }]).map(p => <option key={p.mode} value={p.mode} disabled={!p.available}>{providerLabel(p.mode)}{p.available ? "" : " · not configured"}</option>)}</ValidatedSelect><button type="button" className={primary} disabled={busy || pending || !readyForGeneration} onClick={() => void generate()}><PlusIcon className="h-5 w-5" />{busy ? "Queuing…" : "Generate TSC draft"}</button></div> : null}</header>
  <div className="rounded-xl border border-slate-200 p-3 text-sm dark:border-slate-700" aria-label="Tender cloud allowance"><p className="font-semibold">Cloud allowance: ₹{(options.reserved_paise / 100).toFixed(2)} reserved / ₹{(options.limit_paise / 100).toFixed(2)}</p><p className="text-xs">Conservative request allowances, not invoiced usage. Shared across generations and question research. Source search is free. Paid calls never switch providers or retry automatically.</p>{options.providers.find(p => p.mode === selectedMode)?.cloud ? <p className="mt-2 text-xs">Generation sends selected source excerpts to {providerLabel(selectedMode)}; each model request reserves ₹{((options.providers.find(p => p.mode === selectedMode)?.allowance_paise || 0) / 100).toFixed(2)}. Completed checklist files are excluded from new evidence snapshots.</p> : null}{optionsError ? <p role="alert">Provider availability could not load: {optionsError}</p> : null}</div>
  {!readyForGeneration ? <p role="status" className="text-sm text-amber-800">Finish document reading in Step 1 before generating the TSC.</p> : null}
  {error ? <p role="alert" className="text-sm text-red-700">{error} <button type="button" className={secondary} onClick={() => void load()}>Refresh jobs</button></p> : null}
  {jobs.length ? <div><ValidatedLabel htmlFor="tsc-job">Generation history (latest 20)</ValidatedLabel><ValidatedSelect id="tsc-job" className={control} value={selectedID} onChange={e => { setSelectedID(e.target.value); setJob(null); }}>{jobs.map(j => <option key={j.id} value={j.id}>{new Date(j.created_at).toLocaleString()} — {providerLabel(j.mode)}</option>)}</ValidatedSelect></div> : <p>No TSC drafts yet. Finish source reading, then generate a draft.</p>}
  {job ? <><p role="status" className="text-sm">Generation: {job.state.replaceAll("_", " ")}</p>{pending ? <p className="text-sm">The worker continues after this panel closes. If the job remains queued, check that the TSC worker is running.</p> : null}
  {job.error ? <p role="alert" className="text-sm text-red-700">Generation stopped: {job.error.replaceAll("_", " ")}. The previous source evidence is retained; correct the issue and start a new draft.</p> : null}
  {stale ? <p role="alert" className="text-sm text-amber-800">Sources changed. This snapshot is retained for history; generate a new draft before editing or exporting.</p> : null}
  <details className="rounded-xl border border-slate-200 p-3 text-sm dark:border-slate-700"><summary className="cursor-pointer font-semibold">Source evidence and processing details</summary><div className="mt-3 space-y-3"><p>Processing attempt {job.attempts}/3{job.model ? ` · ${job.model}` : ""}</p>{job.input.coverage?.version ? <div className="space-y-1 rounded-xl border p-3" aria-label="Evidence processing coverage"><p className="font-semibold">Source preparation</p><p>{job.input.coverage.documents} documents · {job.input.coverage.segments} segments · {job.input.coverage.blocks} evidence blocks</p><p>{job.input.coverage.suppressed} repeated page furniture blocks retained at lower search priority · {job.input.coverage.empty_segments} empty segments · {job.input.coverage.low_confidence} low-confidence segments</p><p className="text-xs">Reading coverage and answer accuracy are separate. Dates, conditions and amendment precedence require source review.</p></div> : <p>This historical snapshot uses the earlier evidence pipeline. Generate a new draft to apply structured source preparation.</p>}
  {job.result?.warnings?.map((warning, i) => <p key={i} className="text-sm text-amber-800">{warning}</p>)}{canExport && !stale && job.state === "ready" ? <a className={secondary} href={`/api/tender-fms/${base}/${job.id}/evidence`}>Download source evidence MD</a> : null}</div></details>
  {job.state === "ready" ? <><div className="flex flex-wrap items-center justify-between gap-3"><p>{job.input.questions.length} checklist questions · Draft for review</p>{canExport && !stale ? <a className={secondary} href={`/api/tender-fms/${base}/${job.id}/document`}>Download draft DOCX</a> : null}</div><TSCReview key={job.id} job={job} draftID={draftID} editable={canWrite && !stale} saved={setJob} providers={options.providers} refreshBudget={() => void loadOptions()} /></> : null}</> : null}
 </section>;
}

type Correction = Answer;
function TSCReview({ job, draftID, editable, saved, providers, refreshBudget }: { job: Job; draftID: string; editable: boolean; saved: (job: Job) => void; providers: ProviderOptions["providers"]; refreshBudget: () => void }) {
 const [changes, setChanges] = useState<Record<string, Correction>>({});
 const [active, setActive] = useState(job.input.questions[0]?.id || ""), [researchQuestion, setResearchQuestion] = useState("");
 const [search, setSearch] = useState(""), [documentID, setDocumentID] = useState("");
 const [questionSearch, setQuestionSearch] = useState(""), [stateFilter, setStateFilter] = useState("");
 const [saving, setSaving] = useState(""), [notice, setNotice] = useState("");
 const [errors, setErrors] = useState<Record<string, string>>({});
 const savingRef = useRef(false), reader = useRef<HTMLPreElement>(null);
 const documents = [...new Map(job.input.passages.map(p => [p.document_id, p.filename])).entries()];
 function current(id: string): Correction { const stored = job.result.answers.find(a => a.question_id === id); return changes[id] || { ...(stored || { question_id: id, status: "needs_review", citations: [], origin: "manual" }), answer: editableTSCAnswer(stored), accepted: stored?.accepted || false }; }
 function update(id: string, patch: Partial<Correction>) { if (current(id).accepted) { setNotice("Uncheck Accepted before changing this answer."); return; } setChanges(previous => ({ ...previous, [id]: { ...current(id), ...patch } })); setErrors(previous => ({ ...previous, [id]: "" })); setNotice(""); }
 const normalize = (text: string) => text.toLowerCase().replace(/\s+/g, " ").trim();
 const passages = job.input.passages.filter(p => (!documentID || p.document_id === documentID) && (!search.trim() || normalize(`${p.heading || ""} ${p.text} ${p.filename} ${p.locator}`).includes(normalize(search))));
 const markdown = "# Consolidated extracted documents\n\n" + passages.map(p => `## ${p.filename}\n\n### ${p.locator}${p.heading ? ` — ${p.heading}` : ""}\n\n${p.text}\n`).join("\n");
 const questions = job.input.questions.filter(q => (!questionSearch || normalize(`${q.question} ${q.section} ${current(q.id).answer}`).includes(normalize(questionSearch))) && (!stateFilter || (stateFilter === "accepted" ? current(q.id).accepted : stateFilter === "unaccepted" ? !current(q.id).accepted : current(q.id).status === stateFilter)));
 async function save(id: string, accepted: boolean) {
  if (savingRef.current || !editable) return; const answer = current(id);
  if (accepted && placeholderTSCText(answer.answer)) { setErrors(e => ({ ...e, [id]: "Enter an answer before accepting." })); return; }
  if (accepted && answer.status !== "answered") { setErrors(e => ({ ...e, [id]: "Resolve the source or company-review gap before accepting this answer." })); return; }
  if (accepted && !answer.citations.length) { setErrors(e => ({ ...e, [id]: "Select supporting text in the Markdown document and attach it to this question." })); return; }
  savingRef.current = true; setSaving(id); setNotice("");
  try { const result = await call<Job>(`drafts/${draftID}/tsc/${job.id}/answers/${id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ expected_version: job.version, status: answer.status, answer: answer.answer, citations: answer.citations, accepted }) }); saved(result); setChanges(previous => { const next = { ...previous }; delete next[id]; return next; }); setNotice(accepted ? "Answer accepted." : "Answer reopened."); }
  catch (e) { setErrors(previous => ({ ...previous, [id]: message(e) })); } finally { savingRef.current = false; setSaving(""); }
 }
 function attachSelection() {
  const selection = window.getSelection(); if (!selection || !reader.current?.contains(selection.anchorNode) || !reader.current.contains(selection.focusNode)) { setNotice("Select supporting text inside the Markdown document first."); return; }
  const quote = selection.toString().trim(), source = job.input.passages.find(p => p.text.includes(quote));
  if (quote.length < 5 || quote.length > 2000 || !source) { setNotice("Select 5–2000 characters within one source passage, excluding Markdown headings."); return; }
  const answer = current(active); if (answer.citations.length >= 8) { setNotice("This question already has eight citations."); return; }
  if (!answer.citations.some(c => c.source_id === source.id && c.quote === quote)) update(active, { citations: [...answer.citations, { source_id: source.id, quote }] }); setNotice("Selected source text attached to the active question.");
 }
 return <section aria-label="TSC split review" className="grid min-w-0 gap-4 lg:grid-cols-2">
 <section aria-label="Editable TSC" className="flex h-[78vh] min-h-[500px] min-w-0 flex-col rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
 <header className="space-y-2 border-b border-slate-200 p-3 dark:border-slate-700"><h3 className="font-bold">Tender Selection Checklist</h3><ValidatedLabel htmlFor="tsc-question-search">Filter TSC questions</ValidatedLabel><ValidatedInput id="tsc-question-search" className={control} value={questionSearch} onChange={e => setQuestionSearch(e.target.value)} /><ValidatedLabel htmlFor="tsc-state-filter">Answer filter</ValidatedLabel><ValidatedSelect id="tsc-state-filter" className={control} value={stateFilter} onChange={e => setStateFilter(e.target.value)}><option value="">All answers</option><option value="accepted">Accepted</option><option value="unaccepted">Not accepted</option>{["answered", "not_found", "needs_review", "conflicting"].map(s => <option key={s} value={s}>{s.replaceAll("_", " ")}</option>)}</ValidatedSelect><p className="text-xs">{questions.length} of {job.input.questions.length} questions · {job.result.answers.filter(a => a.accepted).length} accepted · {Object.keys(changes).length} unsaved</p></header>
 <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-3">{questions.map(q => { const answer = current(q.id); return <ValidatedForm key={q.id} aria-label={`TSC question ${q.id}`} onFocusCapture={() => setActive(q.id)} onSubmit={e => e.preventDefault()} className={`space-y-3 rounded-xl border p-3 ${active === q.id ? "border-blue-600" : "border-slate-200 dark:border-slate-700"}`}>
 <h4 className="font-semibold">{q.printed_number} {q.question}</h4><p className="text-xs text-slate-500">{q.section.replaceAll("_", " ")}</p>
 <button type="button" disabled={!!answer.accepted} className={secondary} onClick={() => setResearchQuestion(previous => previous === q.id ? "" : q.id)}>Search sources for this question</button>
 {researchQuestion === q.id && !answer.accepted ? <TSCResearchPanel draftID={draftID} jobID={job.id} questionID={q.id} version={job.version} providers={providers} editable={editable} refreshBudget={refreshBudget} adopt={proposal => { update(q.id, { ...proposal, accepted: false }); setNotice("Proposal copied to the answer editor. Review it, then check Accepted."); }} attach={p => { const a = current(q.id); if (a.citations.length >= 8) { setNotice("At most eight citations per answer."); return; } if (!a.citations.some(c => c.source_id === p.id)) update(q.id, { citations: [...a.citations, { source_id: p.id, quote: p.text }] }); }} /> : null}
 <fieldset disabled={!editable || !!saving || !!answer.accepted} className="space-y-3"><ValidatedLabel htmlFor={`answer-${q.id}`}>Answer</ValidatedLabel><ValidatedTextarea id={`answer-${q.id}`} maxLength={8000} rows={2} className={control} value={answer.answer} onChange={e => { const text = e.target.value; const decision = q.id.startsWith("conclusion.") || q.section === "work_location" || ["risk.q01", "risk.q02", "risk.q03"].includes(q.id); const label = text.trim().toLowerCase(); const status = decision || label === "needs review" ? "needs_review" : label === "not found" ? "not_found" : label === "conflicting information" ? "conflicting" : "answered"; update(q.id, { answer: text, status }); }} />
 <details><summary className="cursor-pointer text-sm">Supporting evidence ({answer.citations.length})</summary>{answer.citations.map((c, i) => <div key={i} className="my-2 space-y-2 border-l-2 border-blue-300 pl-2 text-xs"><p>{job.input.passages.find(p => p.id === c.source_id)?.filename}</p><blockquote className="whitespace-pre-wrap break-words">{c.quote}</blockquote><button type="button" className={secondary} onClick={() => update(q.id, { citations: answer.citations.filter((_, index) => index !== i) })}>Remove citation</button></div>)}</details>
 </fieldset>
 <ValidatedLabel className="inline-flex min-h-11 cursor-pointer items-center gap-3 text-sm font-semibold"><ValidatedInput type="checkbox" className="h-5 w-5 accent-blue-700" checked={!!answer.accepted} disabled={!editable || !!saving} onChange={e => void save(q.id, e.target.checked)} />Accepted</ValidatedLabel>
 {answer.accepted ? <p className="text-xs text-emerald-800 dark:text-emerald-300">Accepted for this checklist. Uncheck to edit or request another proposal.</p> : null}
 {errors[q.id] ? <p role="alert" className="text-sm text-red-700">{errors[q.id]}</p> : null}
 {saving === q.id ? <p role="status" className="text-sm">Saving…</p> : null}
 </ValidatedForm>; })}{!questions.length ? <p>No questions match these filters.</p> : null}</div></section>
 <aside aria-label="Consolidated Markdown reader" className="flex h-[78vh] min-h-[500px] min-w-0 flex-col rounded-xl border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900"><header className="space-y-2 border-b border-slate-200 p-3 dark:border-slate-700"><h3 className="font-bold">Consolidated extracted documents.md</h3><ValidatedLabel htmlFor="tsc-md-search">Search consolidated Markdown</ValidatedLabel><ValidatedInput id="tsc-md-search" className={control} value={search} onChange={e => setSearch(e.target.value)} /><ValidatedLabel htmlFor="tsc-md-document">Filter source document</ValidatedLabel><ValidatedSelect id="tsc-md-document" className={control} value={documentID} onChange={e => setDocumentID(e.target.value)}><option value="">All documents</option>{documents.map(([id, name]) => <option key={id} value={id}>{name}</option>)}</ValidatedSelect><p role="status" className="text-xs">{passages.length} of {job.input.passages.length} source sections · {search || documentID ? "Filtered document" : "Complete consolidated document"}</p>{editable ? <button type="button" className={secondary} onMouseDown={e => e.preventDefault()} onClick={attachSelection}>Attach selected text to active question</button> : null}<p className="text-xs">Active question: {job.input.questions.find(q => q.id === active)?.question}</p>{notice ? <p role="status" className="text-sm text-blue-700 dark:text-blue-300">{notice}</p> : null}</header><pre ref={reader} aria-label="Consolidated Markdown document" tabIndex={0} className="min-h-0 flex-1 overflow-y-auto whitespace-pre-wrap break-words p-4 font-mono text-sm [overflow-wrap:anywhere]">{passages.length ? markdown : "No source text matches these filters."}</pre></aside>
 </section>;
}
