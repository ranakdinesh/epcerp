"use client";

import { useRef, useState } from "react";
import { ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";

export type ProviderOption = { mode: string; model: string; available: boolean; cloud: boolean; allowance_paise: number };
export type ProviderOptions = { providers: ProviderOption[]; limit_paise: number; reserved_paise: number };
type Passage = { id: string; filename: string; locator: string; text: string; method?: string; confidence?: number };
type Proposal = { question_id: string; answer: string; status: string; origin: string; citations: { source_id: string; quote: string }[] };
type Research = { passages: Passage[]; proposal?: Proposal; warning: string };
const control = "min-h-11 w-full rounded-xl border border-slate-300 bg-white p-2 text-sm dark:border-slate-600 dark:bg-slate-900";
const button = "min-h-11 rounded-xl border border-slate-300 px-3 text-sm font-semibold disabled:opacity-50";
export const providerLabel = (mode: string) => ({ gemini: "Gemini Flash-Lite", openai: "OpenAI API · GPT-6 Luna", ollama: "Local AI · development", manual: "Manual blank checklist" }[mode] || mode);

export function TSCResearchPanel({ draftID, jobID, questionID, version, providers, editable, adopt, attach, refreshBudget }: {
  draftID: string; jobID: string; questionID: string; version: number; providers: ProviderOption[]; editable: boolean;
  adopt: (proposal: Proposal) => void; attach: (passage: Passage) => void; refreshBudget: () => void;
}) {
  const [query, setQuery] = useState(""), [mode, setMode] = useState(""), [result, setResult] = useState<Research | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState("");
  const operation = useRef<{ fingerprint: string; key: string } | null>(null);
  const available = providers.filter(p => p.available && p.mode !== "manual");
  const selected = available.find(p => p.mode === mode) || available[0];
  async function research(paid: boolean) {
    if (busy) return;
    setBusy(true); setError("");
    const requestedMode = paid ? selected?.mode : "search";
    const fingerprint = `${version}:${query}:${requestedMode}`;
    if (operation.current?.fingerprint !== fingerprint) operation.current = { fingerprint, key: crypto.randomUUID() };
    try {
      const response = await fetch(`/api/tender-fms/drafts/${draftID}/tsc/${jobID}/answers/${questionID}/research`, {
        method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": operation.current.key },
        body: JSON.stringify({ expected_version: version, query, mode: requestedMode }),
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
    {editable && available.length ? <><ValidatedLabel htmlFor={`research-provider-${questionID}`}>Proposal provider</ValidatedLabel><ValidatedSelect id={`research-provider-${questionID}`} className={control} value={selected?.mode || ""} onChange={e => setMode(e.target.value)} disabled={busy}>{available.map(p => <option key={p.mode} value={p.mode}>{providerLabel(p.mode)}</option>)}</ValidatedSelect>
      <p className="text-xs">{selected?.cloud ? `Sends bounded source excerpts to ${providerLabel(selected.mode)}. Reserves ₹${(selected.allowance_paise / 100).toFixed(2)} from this tender’s ₹100 allowance, including failed or uncertain requests.` : "Uses the development-only local model."}</p>
      <button type="button" className={button} disabled={busy} onClick={() => void research(true)}>Request revised proposal</button></> : <p className="text-xs">Source search works without a model. Cloud proposals need an approved provider and a server-side API key.</p>}
    {error ? <p role="alert" className="text-sm text-red-700">{error}</p> : null}
    {result ? <><p className="text-xs text-amber-800 dark:text-amber-300">{result.warning}</p>
      {result.proposal ? <div className="space-y-2 rounded-xl border p-3"><p className="font-semibold">Proposed answer · {result.proposal.status.replaceAll("_", " ")}</p><p className="whitespace-pre-wrap break-words">{result.proposal.answer}</p><p className="text-xs">{result.proposal.citations.length} source citations. Review the originals; a valid citation does not prove the interpretation is correct.</p><button type="button" className={button} onClick={() => adopt(result.proposal!)}>Use proposal in answer</button><p className="text-xs">Replaces this question’s unsaved text and citations. Review it, then check Accepted to save.</p></div> : null}
      <p className="text-sm">{result.passages.length} relevant excerpts</p>
      {result.passages.map(p => <details key={p.id} className="rounded-xl border p-2"><summary className="cursor-pointer break-words text-xs">{p.filename} · {p.locator}</summary><p className="my-2 whitespace-pre-wrap break-words text-sm">{p.text}</p>{p.confidence !== undefined ? <p className="text-xs">OCR confidence {p.confidence.toFixed(0)} · not answer accuracy</p> : null}{editable ? <button type="button" className={button} onClick={() => attach(p)}>Attach this excerpt</button> : null}</details>)}
    </> : null}
  </div>;
}
