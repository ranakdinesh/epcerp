import type { Processing } from "@/server/tenders/tsc/types";
import { generationLabel, processingElapsed, processingFailure, processingTitle, providerName } from "./tsc-processing-model";

type Generation = { state: string; mode: string; error_code?: string; processing?: Processing };

export function TSCGenerationProgress({ job, submitting, statusUnavailable = false }: { job?: Generation | null; submitting: boolean; statusUnavailable?: boolean }) {
  if (!submitting && (!job || job.mode === "manual")) return null;

  const processing = job?.processing;
  const provider = job ? providerName(job.mode) : "the model";
  const pending = submitting || job?.state === "queued" || job?.state === "processing";
  const title = submitting ? "Submitting checklist request…" : processing ? processingTitle(processing, provider) : job && generationLabel(job);
  const error = !submitting && job?.error_code;

  return <section aria-label="Background scrutiny" className="space-y-2 rounded-xl border border-slate-200 bg-slate-50 p-3 text-sm dark:border-slate-700 dark:bg-slate-800/70">
    <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-1">
      <p className="font-semibold text-slate-900 dark:text-slate-100" role="status">{title}</p>
      {!submitting && processing && <span className="text-sm tabular-nums text-slate-600 dark:text-slate-300">{processingElapsed(processing.elapsed_seconds)} elapsed</span>}
    </div>
    {pending && statusUnavailable && <p className="text-sm text-slate-600 dark:text-slate-300">Showing the last confirmed stage while the status reconnects.</p>}
    {pending && !statusUnavailable && <div role="progressbar" aria-label="Background scrutiny in progress" className="h-1.5 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-700">
      <div aria-hidden="true" className="h-full w-1/3 rounded-full bg-[var(--action-color)] motion-safe:animate-pulse" />
    </div>}
    {!submitting && processing && processing.model_questions > 0 && <p className="text-sm text-slate-600 dark:text-slate-300">{processing.returned_answers} of {processing.model_questions} model results saved</p>}
    {error && <p className="text-sm text-red-700 dark:text-red-300" role="alert">{processingFailure(error, provider)}</p>}
  </section>;
}
