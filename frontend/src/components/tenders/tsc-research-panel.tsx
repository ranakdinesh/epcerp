"use client";
import { createRecordId } from "@/lib/record-id";

import type { TSCFormat } from "./tsc-format-model";
import { useRef, useState } from "react";
import { ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";

export type ProviderOption = { mode: string; model: string; available: boolean; cloud: boolean; allowance_paise: number; original_documents?: boolean; original_allowance_paise?: number };
export type ProviderOptions = { formats?: TSCFormat[]; providers: ProviderOption[]; limit_paise: number; reserved_paise: number };
type Passage = { id: string; filename: string; locator: string; text: string; method?: string; confidence?: number };
type Proposal = { question_id: string; answer: string; status: string; origin: string; citations: { source_id: string; quote: string }[] };
type Research = { retrieval?: { method?: string; fallback?: string; selected_bytes?: number; baseline_bytes?: number; context_gap?: boolean }; passages: Passage[]; proposal?: Proposal; warning: string };
const control = "min-h-11 w-full rounded-xl border border-slate-300 bg-white p-2 text-sm dark:border-slate-600 dark:bg-slate-900";
const button = "min-h-11 rounded-xl border border-slate-300 px-3 text-sm font-semibold disabled:opacity-50";
export const providerLabel = (mode: string) => ({ local: "Local extraction · no LLM", gemini: "Gemini Flash-Lite", gemini_originals: "Gemini · original source PDFs", openai: "OpenAI API · GPT-6 Luna", ollama: "Local AI · development", manual: "Manual blank checklist" }[mode] || mode);

export function TSCResearchPanel({ draftID, jobID, questionID, version, providers, editable, adopt, attach, refreshBudget }: {
  draftID: string; jobID: string; questionID: string; version: number; providers: ProviderOption[]; editable: boolean;
  adopt: (proposal: Proposal) => void; attach: (passage: Passage) => void; refreshBudget: () => void;
}) {
  const [query, setQuery] = useState(""), [mode, setMode] = useState(""), [result, setResult] = useState<Research | null>(null);
  const [originalsConfirmed, setOriginalsConfirmed] = useState(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState("");
  const operation = useRef<{ fingerprint: string; key: string } | null>(null);
  const baseProviders = providers.filter(p => p.available && p.mode !== "manual" && p.mode !== "local");
  const gemini = baseProviders.find(p => p.mode === "gemini" && p.original_documents);
  const available = gemini ? [...baseProviders, { ...gemini, mode: "gemini_originals", allowance_paise: gemini.original_allowance_paise || 500 }] : baseProviders;
  const selected = available.find(p => p.mode === mode) || available[0];
  const originals = selected?.mode === "gemini_originals";
  async function research(paid: boolean) {
    if (busy || (paid && originals && !originalsConfirmed)) return;
    setBusy(true); setError("");
    const requestedMode = paid ? selected?.mode : "search";
    const fingerprint = `${version}:${query}:${requestedMode}`;
    try {
      if (operation.current?.fingerprint !== fingerprint) operation.current = { fingerprint, key: createRecordId() };
      const response = await fetch(`/api/tender-fms/drafts/${draftID}/tsc/${jobID}/answers/${questionID}/research`, {
        method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": operation.current.key },
        body: JSON.stringify({ expected_version: version, query, mode: requestedMode, original_documents_confirmed: paid && originals && originalsConfirmed }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body.detail || "Question research could not complete");
      setResult(body);
    } catch (e) { setError(e instanceof Error ? e.message : "Question research could not complete"); }
    finally { setBusy(false); refreshBudget(); }
  }
  return <div className="space-y-3 rounded-xl border border-blue-200 p-3 dark:border-blue-800" aria-label={`Research ${questionID}`}>
    <p className="font-semibold">Search this question in retained sources</p>
    <ValidatedLabel htmlFor={`research-query-${questionID}`}>Additional search terms (optional)</ValidatedLabel>
    <ValidatedInput id={`research-query-${questionID}`} maxLength={300} className={control} value={query} onChange={e => { setQuery(e.target.value); setResult(null); }} disabled={busy} />
    <button type="button" className={button} disabled={busy} onClick={() => void research(false)}>{busy ? "Searching…" : "Search sources · no cloud cost"}</button>
    {editable && available.length ? <><ValidatedLabel className="block" htmlFor={`research-provider-${questionID}`}>Proposal provider</ValidatedLabel><ValidatedSelect id={`research-provider-${questionID}`} className={control} value={selected?.mode || ""} onChange={e => { setMode(e.target.value); setOriginalsConfirmed(false); setResult(null); }} disabled={busy}>{available.map(p => <option key={p.mode} value={p.mode}>{providerLabel(p.mode)}</option>)}</ValidatedSelect>
      {originals ? <div className="space-y-2 rounded-xl border border-amber-300 p-3 text-sm dark:border-amber-700"><p>Reads all retained source PDFs with Gemini for this question. Supports up to 8 PDFs, 50 pages and 8 MiB. The request stops if the complete package cannot be handled; files are not silently dropped. Completed checklist/reference files remain excluded.</p><ValidatedLabel htmlFor={`original-confirm-${questionID}`} className="flex min-h-11 items-start gap-3"><ValidatedInput id={`original-confirm-${questionID}`} type="checkbox" className="mt-1 h-5 w-5" checked={originalsConfirmed} onChange={e => setOriginalsConfirmed(e.target.checked)} disabled={busy} />I agree to send these original source PDFs to Google for this proposal.</ValidatedLabel></div> : null}
      <p className="text-xs">{originals ? `Reserves ₹${((selected?.allowance_paise || 500) / 100).toFixed(2)} from this tender’s ₹100 allowance, including failed or uncertain requests. This is a conservative allowance, not invoiced usage. No automatic paid retry.` : selected?.cloud ? `Sends bounded source excerpts to ${providerLabel(selected.mode)}. Reserves ₹${(selected.allowance_paise / 100).toFixed(2)} from this tender’s ₹100 allowance, including failed or uncertain requests.` : "Uses the development-only local model."}</p>
      <button type="button" className={button} disabled={busy || (originals && !originalsConfirmed)} onClick={() => void research(true)}>{originals ? "Read original PDFs with Gemini" : "Request revised proposal"}</button></> : <p className="text-xs">Source search works without a model. Cloud proposals need an approved provider and a server-side API key.</p>}
    {error ? <p role="alert" className="text-sm text-red-700">{error}</p> : null}
    {result ? <>{result.retrieval ? <p className="text-xs">Evidence selection: {result.retrieval.method === "original_pdf_package" ? "Original PDF package" : result.retrieval.method === "bm25_minilm" ? "Local relevance ranking" : "Keyword search"} · {result.retrieval.selected_bytes || 0} bytes{result.retrieval.fallback ? " · local ranking unavailable; keyword results retained" : ""}{result.retrieval.context_gap ? " · full-source review needed" : ""}</p> : null}<p className="text-xs text-amber-800 dark:text-amber-300">{result.warning}</p>
      {result.proposal ? <div className="space-y-2 rounded-xl border p-3"><p className="font-semibold">Proposed answer · {result.proposal.status.replaceAll("_", " ")}</p><p className="whitespace-pre-wrap break-words">{result.proposal.answer}</p><p className="text-xs">{result.proposal.citations.length} source citations. Review the originals; a valid citation does not prove the interpretation is correct.</p><button type="button" className={button} onClick={() => adopt(result.proposal!)}>Use proposal in answer</button><p className="text-xs">Replaces this question’s unsaved text and citations. Review it, then check Accepted to save.</p></div> : null}
      <p className="text-sm">{result.passages.length} relevant excerpts</p>
      {result.passages.map(p => <details key={p.id} className="rounded-xl border p-2"><summary className="cursor-pointer break-words text-xs">{p.filename} · {p.locator}</summary><p className="my-2 whitespace-pre-wrap break-words text-sm">{p.text}</p>{p.confidence !== undefined ? <p className="text-xs">OCR confidence {p.confidence.toFixed(0)} · not answer accuracy</p> : null}{editable ? <button type="button" className={button} onClick={() => attach(p)}>Attach this excerpt</button> : null}</details>)}
    </> : null}
  </div>;
}
