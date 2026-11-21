import type { Answer, Question } from "@/server/tenders/tsc/types";
export type { Processing } from "@/server/tenders/tsc/types";
import type { Processing } from "@/server/tenders/tsc/types";
export function generationLabel(job: {state:string;mode:string;error_code?:string;processing?:Pick<Processing,"phase">;preparation_method?:string;snapshot?:{preparation_method?:string}}) {
  if (job.state === "queued") return "Queued";
  if (job.state === "processing") return "Processing";
  if (job.error_code) return job.processing?.phase === "partial" ? "Partial draft — answers saved" : "Generation failed";
  if(job.preparation_method==="upload"||job.snapshot?.preparation_method==="upload")return "Uploaded Word";
  return job.mode === "manual" ? "Manual draft" : "Generation finished";
}
export function processingTitle(p: Processing, provider: string) {
  switch (p.phase) {
    case "queued": return "Waiting to start";
    case "preparing_documents": return `Preparing ${p.source_count} source document${p.source_count === 1 ? "" : "s"} for ${provider}`;
    case "waiting_for_model": return `Waiting for ${provider} to answer ${p.model_questions} questions`;
    case "retrying": return `Recovering from temporary ${provider} overload`;
    case "saving_answers": return "Saving the model response";
    case "failed": return "Generation failed — no answers received";
    case "partial": return "Processing stopped — some answers were saved";
    case "imported": return "Word checklist saved";
    case "manual": return "Manual draft — no model request";
    default: return p.returned_answers ? "Generation finished" : "Generation finished — no usable answers returned";
  }
}
export function processingFailure(code: string, provider: string) {
  if (code === "provider_unavailable") return `${provider} returned a service error (HTTP 5xx). The request stopped; no usable answers were returned by this attempt.`;
  if (code === "provider_outcome_uncertain" || code === "worker_interrupted") return "The request ended without a confirmed outcome. Saved responses are retained. Review the saved status before starting a new request.";
  if(code === "tender_allowance_reached")return "The remaining tender allowance cannot cover the next model request. Saved answers remain available; no extra generation request was sent.";
  if(code === "token_count_failed")return "The provider could not confirm the input size. Generation was not sent; the allowance was preserved.";
  if (code === "quota_or_rate_limit") return `${provider} rejected the request because its quota or rate limit was reached.`;
  if (code === "provider_access_denied") return `${provider} rejected the configured API credentials or access.`;
  if (code === "invalid_provider_answers" || code === "invalid_provider_response" || code === "incomplete_provider_response") return "The model response could not be used as a complete checklist. Its original response is retained for inspection.";
  if(code === "document_text_extraction_failed")return "Local PDF text extraction failed before sending a model request. Check that the document reader is available.";
  if(code === "source_context_exceeded")return "The selected evidence exceeds this model’s context limit. Select fewer documents or use a larger-context provider.";
  if (code === "document_upload_failed" || code === "document_processing_failed" || code === "document_processing_timeout") return "Source document preparation failed before a model answer was received.";
  return `Processing stopped: ${code.replaceAll("_", " ")}. Saved answers remain available.`;
}
export function questionProgress(q: Question, answer: Answer | undefined, pending: boolean, failed: boolean, provider: string) {
  if (answer?.accepted) return "Reviewed";
  if (answer?.answer.trim()) return answer.status === "conflicting" ? "Conflicting evidence" : answer.status === "answered" ? "Answer ready to review" : "Answer needs review";
  if (q.manual_review) return "Manager judgement required";
  if (pending) return `Waiting for ${provider}`;
  if (failed) return "No answer received";
  if (answer?.status === "not_found") return "Not found in sources";
  return "Not filled in";
}

export const providerName=(mode:string)=>({gemini:"Gemini",openai:"OpenAI",deepseek:"DeepSeek",groq:"Groq",manual:"Manual writing"}[mode]??mode);
