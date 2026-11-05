import { AppError } from '../../errors';
import type { Answer, Citation, Mode, Question, Snapshot, Source, Provider } from './types';
export const uuid = /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/;
export function invalid(message: string): never { throw new AppError(422, message); }
export function object(value: unknown, fields: string[]): Record<string, unknown> { if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(k => !fields.includes(k)))
    invalid('Invalid request fields'); return value as Record<string, unknown>; }
export function str(value: unknown, max = 2000) { if (typeof value !== 'string' || Buffer.byteLength(value) > max)
    invalid('Provide bounded text'); return value as string; }
export function integer(value: unknown, min = 0, max = Number.MAX_SAFE_INTEGER) { if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < min || value > max)
    invalid('Provide a valid version or number'); return value as number; }
export function flag(value: unknown) { if (typeof value !== 'boolean')
    invalid('Provide a boolean value'); return value as boolean; }
export function id(value: unknown) { const s = str(value, 36); if (!uuid.test(s))
    invalid('Invalid identifier'); return s; }
export function mode(value: unknown): Mode { if (!['manual', 'gemini', 'openai', 'deepseek', 'groq'].includes(value as string))
    invalid('Choose a configured provider or manual writing'); return value as Mode; }
export function sourceSelection(values: unknown, sources: Source[], mode: Mode) { if (!Array.isArray(values) || values.length > 30 || values.some(v => typeof v !== 'string') || new Set(values).size !== values.length)
    invalid('Choose a valid source document list'); const selected = values.map(v => { const s = sources.find(s => s.id === v); if (!s)
    invalid('A selected document is no longer available'); return s!; }); if (mode !== 'manual') {
    if (!selected.length)
        invalid('Choose source PDFs or use manual writing');
    if (selected.some(s => s.kind !== 'pdf'))
        invalid('Direct cloud reading currently requires PDF sources. Export selected Office documents to PDF, or use manual writing.');
    if (selected.reduce((n, s) => n + s.byte_size, 0) > 40 * 1024 * 1024 || selected.reduce((n, s) => n + s.pages, 0) > 1000)
        invalid('Selected source package exceeds 40 MiB or 1,000 PDF pages; choose a smaller explicit document set');
} return selected; }
export const blank = (q: Question): Answer => ({ question_id: q.id, answer: '', status: 'needs_review', citations: [], accepted: false, origin: 'manual' });
export function citations(value: unknown, sources: Source[]): Citation[] { if (!Array.isArray(value) || value.length > 8)
    invalid('Use at most eight document references'); return value.map(v => { const c = object(v, ['document_id', 'page', 'quote']), document_id = id(c.document_id), source = sources.find(s => s.id === document_id); if (!source)
    invalid('Reference only selected tender documents'); const page = integer(c.page, 1, source!.pages || 1000), quote = str(c.quote, 2000); if (!quote.trim())
    invalid('Provide a source quotation'); return { document_id, page, quote }; }); }
export function managerAnswer(value: unknown, snapshot: Snapshot): Answer { const r = object(value, ['question_id', 'answer', 'status', 'citations', 'accepted', 'origin']), question_id = str(r.question_id, 80); if (!snapshot.questions.some(q => q.id === question_id))
    invalid('Unknown TSC question'); const answer = str(r.answer, 8000), status = str(r.status, 30); if (!['answered', 'needs_review', 'not_found', 'conflicting'].includes(status))
    invalid('Invalid answer status'); const accepted = flag(r.accepted); if (accepted && !answer.trim())
    invalid('Write an answer before marking it reviewed'); return { question_id, answer, status: status as Answer['status'], citations: citations(r.citations, snapshot.sources), accepted, origin: 'manual' }; }
// Model citations are syntax/source checked, never represented as independently verified quotations.
export function providerAnswers(value: unknown, questions: Question[], sources: Source[], provider: Provider): Answer[] { const r = object(value, ['answers']); if (!Array.isArray(r.answers) || r.answers.length > questions.length)
    invalid('Unexpected model answer list'); const seen = new Set<string>(), out: Answer[] = []; for (const value of r.answers) {
    const a = object(value, ['question_id', 'answer', 'status', 'citations']), qid = str(a.question_id, 80);
    if (seen.has(qid) || !questions.some(q => q.id === qid))
        invalid('Model returned unknown or duplicate questions');
    seen.add(qid);
    let status = str(a.status, 30) as Answer['status'], refs: Citation[] = [];
    const answer = str(a.answer, 8000);
    if (!['answered', 'needs_review', 'not_found', 'conflicting'].includes(status))
        invalid('Invalid model answer status');
    try {
        refs = citations(a.citations, sources);
    }
    catch {
        status = 'needs_review';
    }
    if (status === 'answered' && (!refs.length || !answer.trim()))
        status = 'needs_review';
    out.push({ question_id: qid, answer, status, citations: refs, accepted: false, origin: provider });
} for (const q of questions)
    if (!seen.has(q.id))
        out.push({ ...blank(q), origin: provider }); return out; }
