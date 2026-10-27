export function placeholderTSCText(text: string): boolean {
 const value = text.trim().toLowerCase().replace(/^[.:;* _\-"`]+|[.:;* _\-"`]+$/g, "").replaceAll("_", " ").replace(/\s+/g, " ");
 return ["", "answered", "answer", "not found", "needs review", "conflicting", "conflicting information", "unknown", "not available", "not yet extracted"].includes(value);
}
export function checklistAnswer(answer?: { status: string; answer: string }): string {
 if (!answer || answer.status !== "answered") return "";
 const text = answer.answer.replace(/^answered\s*:\s*/i, "").trim();
 return placeholderTSCText(text) ? "" : text;
}

// Historical snapshots keep their diagnostics; editors show only answer content.
export function editableTSCAnswer(answer?: {status:string;answer:string;origin?:string;question_id?:string;accepted?:boolean}):string {
  if (!answer || placeholderTSCText(answer.answer.replace(/^answered\s*:\s*/i, ""))) return "";
  if (answer.origin === "manual_draft" || answer.origin === "cloud_review_required" || answer.origin === "retrieval_gap" || answer.origin?.startsWith("rejected_")) return "";
  if (/^(The local model proposal failed|Cloud proposal unavailable|No supporting evidence was retrieved|Not yet extracted\.|Reviewed company\/risk evidence)/.test(answer.answer)) return "";
  if (answer.status === "not_found") return "";
  return answer.status === "answered" && !answer.accepted && answer.origin !== "manual" ? compactTSCText(answer.question_id || "", answer.answer) : answer.answer;
}

// Same conservative presentation rules as domain.CompactTSCText; no source text is changed.
export function compactTSCText(question: string, text: string): string {
  text = text.replace(/\s+/g, " ").replace(/\s*\(S[0-9]+(?:\s*[,;]\s*S[0-9]+)*\)/g, "").replace(/^Answered:/, "").trim();
  const parts = text.split(/[.!?]\s+/), first = parts[0]?.trim() || "";
  if (question === "particulars.q05") {
    const amounts = first.match(/(?:INR|Rs\.?|₹)\s*[0-9][0-9,]*(?:\.[0-9]+)?(?:\s*(?:crores?|lakhs?|lacs?|million|billion))?/gi) || [];
    if (amounts.length === 1 && !/[<>≤≥]/.test(first) && !/\b(?:between|from|approximately|about|upto|up to|at least|more than|less than|or|excluding|including)\b/i.test(first)) return amounts[0].trim();
  }
  const prefix = "Reverse Auction (RA) is applicable to ";
  if (question === "particulars.q06" && first.toLowerCase().startsWith(prefix.toLowerCase())) return ("Yes; " + first.slice(prefix.length)).replaceAll("arranged in order of prices from lowest to highest", "ranked by price (lowest first)").replace(/\.$/, "");
  return parts.map(p => p.trim()).filter((p, i) => p && (i === 0 || !/^(?:this is (?:stated|mentioned|specified)|this (?:is|can be) (?:found|confirmed)|(?:the )?(?:document|source|passage|clause) (?:states|mentions|specifies|confirms)|for (?:example|instance)|as (?:stated|mentioned)|see (?:source|page|clause))\b/i.test(p))).join("; ");
}
