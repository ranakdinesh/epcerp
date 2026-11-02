export type FormatQuestion = { id: string; section: string; printed_number: string; question: string; manual_review?: boolean; row?: number };
export type TSCFormat = { id: string; name: string; description: string; purpose?: string; created_at?: string; is_active: boolean; version: number; question_count: number; questions?: FormatQuestion[]; updated_at?: string };
export const questionKey = (text: string) => text.toLowerCase().replace(/[\p{White_Space}\p{P}]/gu, "");
export function duplicateQuestions(questions: FormatQuestion[]) {
 const first = new Map<string, number>(), duplicates = new Map<string, number>();
 questions.forEach((q, index) => { const key = questionKey(q.question); if (!key) return; const previous = first.get(key); if (previous !== undefined) duplicates.set(q.id, previous + 1); else first.set(key, index); });
 return duplicates;
}
