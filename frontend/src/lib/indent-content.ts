export const indentFields = [
  ["specifications", "Specifications"],
  ["notes", "Important Notes"],
  ["conditions", "Terms and Conditions"],
] as const;

export type IndentRun = { text: string; bold?: boolean; italic?: boolean; underline?: boolean };
export type IndentField = (typeof indentFields)[number][0];
export const indentSpacingOptions = [1, 1.15, 1.5, 2] as const;
export type IndentSpacing = (typeof indentSpacingOptions)[number];
export type IndentContent = Record<IndentField, IndentRun[][]> & {
  spacing?: Partial<Record<IndentField, IndentSpacing>>;
};

export function validateIndentContent(value: unknown): IndentContent | null {
  if (!value || typeof value !== "object") return null;
  const content = value as IndentContent;
  let length = 0;
  let nodes = 0;
  for (const [key] of indentFields) {
    const spacing = content.spacing?.[key];
    if (spacing !== undefined && !indentSpacingOptions.includes(spacing)) return null;
    if (!Array.isArray(content[key])) return null;
    for (const paragraph of content[key]) {
      if (!Array.isArray(paragraph) || ++nodes > 4000) return null;
      for (const run of paragraph) {
        if (++nodes > 4000 || typeof run?.text !== "string" ||
          (run.bold !== undefined && typeof run.bold !== "boolean") ||
          (run.italic !== undefined && typeof run.italic !== "boolean") ||
          (run.underline !== undefined && typeof run.underline !== "boolean")) return null;
        length += run.text.length;
      }
    }
  }
  return length <= 24000 ? content : null;
}

export function readIndentContent(saved: string | undefined, defaultTerms = ""): IndentContent {
  if (saved === undefined) {
    return {
      specifications: [],
      notes: [],
      conditions: defaultTerms ? defaultTerms.split("\n").map(text => text ? [{ text }] : []) : [],
    };
  }
  if (saved.startsWith("indent-rich-v1\n")) {
    try {
      const content = validateIndentContent(JSON.parse(saved.slice("indent-rich-v1\n".length)));
      if (content) return content;
    } catch {}
  }
  return { specifications: saved ? saved.split(/\r?\n/).map(text => [{ text }]) : [], notes: [], conditions: [] };
}

export const indentApprovalLabels = ["Indent Raised by", "Checked by", "Approved by"] as const;
export const indentSignatureNotice = "This is a computer-generated indent and does not require a signature.";
