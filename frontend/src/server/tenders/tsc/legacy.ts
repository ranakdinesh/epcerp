import 'server-only';
import { randomUUID } from 'node:crypto';
import { zipSync, strToU8 } from 'fflate';
import { transaction } from '../../db';
import { authorizeTender } from '../documents';
import { decrypt, encrypt } from './crypto';
import { AppError } from '../../errors';
import { timeSQL, timestamp } from '../../inbox/repository';
import { journal } from '../../purchasing/core/journal';
import { json, body, binary } from '../../purchasing/http-input';
import template from '../formats/template-package.json';
type Citation = {
    source_id: string;
    quote: string;
};
type Answer = {
    question_id: string;
    status: string;
    answer: string;
    citations: Citation[];
    accepted: boolean;
    origin: string;
};
type Passage = {
    id: string;
    filename: string;
    locator: string;
    text: string;
    suppressed?: boolean;
    revision: number;
    sha256: string;
    method?: string;
    kind?: string;
};
type Input = {
    name: string;
    questions: {
        id: string;
        section: string;
        question: string;
        printed_number: string;
    }[];
    passages: Passage[];
    warnings: string[];
    coverage: Record<string, number | string>;
};
type Result = {
    schema: string;
    answers: Answer[];
    warnings: string[];
};
const exact = `${timeSQL('created_at')}AS created_at,${timeSQL('updated_at')}AS updated_at`;
export async function handleLegacy(r: Request, draft: string, job: string, tail: string[], token: string) {
    return transaction(async (db) => {
        const action = r.method === 'PATCH' ? 'write' : ['document', 'evidence'].includes(tail[0]) ? 'export' : 'read', scope = await authorizeTender(db, token, draft, action), row = (await db.query(`SELECT *,${exact} FROM tender.tsc_jobs WHERE id=$1 AND draft_id=$2 FOR UPDATE`, [job, draft])).rows[0];
        if (!row)
            throw new AppError(404, 'TSC draft not found');
        const input = decrypt<Input>(row.input_cipher, row.input_hash);
        let result = row.result_cipher?.length ? decrypt<Result>(row.result_cipher, row.result_hash) : { schema: '', answers: [], warnings: [] };
        const dto = () => ({
            id: row.id, draft_id: row.draft_id, draft_version: Number(row.draft_version), mode: row.mode, model: row.model, state: row.state, error: row.error_code, version: Number(row.version), attempts: row.attempts, created_at: timestamp(row.created_at), updated_at: timestamp(row.updated_at), input, result, stale: Number(row.draft_version) !== scope.draft.version, engine: 'go', read_engine: 'nextjs'
        });
        if (r.method === 'GET' && !tail.length)
            return json(dto());
        if (r.method === 'GET' && tail[0] === 'history')
            return json((await db.query(`SELECT id,action,version,reason,created_by_id,${timeSQL('created_at')}AS created_at FROM tender.tsc_events WHERE job_id=$1 ORDER BY created_at DESC,id DESC`, [job])).rows.map(e => ({ ...e, version: Number(e.version), created_at: timestamp(e.created_at) })));
        if (r.method === 'PATCH' && tail[0] === 'answers' && tail.length === 2) {
            const b = await body(r, ['expected_version', 'status', 'answer', 'citations', 'reason', 'accepted'], 32768);
            if (row.state !== 'ready' || Number(row.draft_version) !== scope.draft.version)
                throw new AppError(409, 'Generate a new draft from current sources before editing');
            if (b.expected_version !== Number(row.version))
                throw new AppError(409, 'TSC draft changed; refresh before saving');
            const previous = result.answers.find(a => a.question_id === tail[1]);
            if (!input.questions.some(q => q.id === tail[1]))
                throw new AppError(422, 'Unknown TSC question');
            if (typeof b.answer !== 'string' || Buffer.byteLength(b.answer) > 8000 || typeof b.status !== 'string' || !['answered', 'not_found', 'needs_review', 'conflicting'].includes(b.status) || typeof b.accepted !== 'boolean' || !Array.isArray(b.citations) || b.citations.length > 8)
                throw new AppError(422, 'Provide a bounded answer and at most eight citations');
            const citations: Citation[] = b.citations.map(c => {
                if (!c || typeof c.source_id !== 'string' || typeof c.quote !== 'string' || Buffer.byteLength(c.quote) < 5 || Buffer.byteLength(c.quote) > 2000 || !input.passages.some(p => p.id === c.source_id && p.text.includes(c.quote)))
                    throw new AppError(422, 'Citation must quote retained source evidence exactly');
                return { source_id: c.source_id, quote: c.quote };
            });
            if (b.status === 'answered' && (['', 'answered', 'answer', 'not found', 'needs review', 'conflicting', 'conflicting information', 'unknown', 'not available', 'not yet extracted'].includes(b.answer.toLowerCase().trim().replace(/^[.:;* _\-"`]+|[.:;* _\-"`]+$/g, '').replaceAll('_', ' ').replace(/\s+/g, ' ')) || !citations.length) || b.accepted && b.status !== 'answered')
                throw new AppError(422, 'An answered question requires source evidence');
            if ((tail[1].startsWith('conclusion.') || ['work_location.q01', 'risk.q01', 'risk.q02', 'risk.q03'].includes(tail[1])) && b.status !== 'needs_review')
                throw new AppError(422, 'This question requires reviewed company/risk evidence and an authorised human decision');
            const same = !!previous && previous.answer === b.answer && previous.status === b.status && previous.citations.length === citations.length && previous.citations.every((c, i) => c.source_id === citations[i].source_id && c.quote === citations[i].quote);
            if (previous?.accepted && !same)
                throw new AppError(409, 'Uncheck Accepted before changing this answer');
            const answer: Answer = {
                question_id: tail[1], status: b.status, answer: b.answer, citations, accepted: b.accepted, origin: same ? previous!.origin : 'manual'
            };
            const reason = typeof b.reason === 'string' ? b.reason : '';
            if (reason && Buffer.byteLength(reason) < 3 || Buffer.byteLength(reason) > 1000)
                throw new AppError(422, 'Provide a correction reason of 3 to 1000 bytes');
            result = { ...result, answers: result.answers.map(a => a.question_id === answer.question_id ? answer : a) };
            const sealed = encrypt(result), changed = (await db.query(`UPDATE tender.tsc_jobs SET result_cipher=$3,result_hash=$4,version=version+1,updated_at=clock_timestamp()WHERE id=$1 AND version=$2 AND state='ready'RETURNING version,${timeSQL('updated_at')}AS updated_at`, [job, row.version, sealed.cipher, sealed.hash])).rows[0];
            if (!changed)
                throw new AppError(409, 'TSC draft changed; refresh before saving');
            row.version = changed.version;
            row.updated_at = changed.updated_at;
            const event = randomUUID(), eventAction = b.accepted ? 'answer_accepted' : previous?.accepted ? 'answer_reopened' : 'answer_corrected';
            await db.query('INSERT INTO tender.tsc_events(id,job_id,action,version,reason,result_cipher,result_hash,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8)', [event, job, eventAction, row.version, reason || 'Reviewer updated checklist answer', sealed.cipher, sealed.hash, scope.actor.id]);
            await journal(db, {
                id: event, module: 'tender_fms', type: 'tsc_draft', entity: job, version: Number(row.version), action: 'tsc_' + eventAction, actor: scope.actor.id, project: null, at: timestamp(row.updated_at)!
            });
            return json(dto());
        }
        if (r.method === 'GET' && ['document', 'evidence'].includes(tail[0])) {
            if (row.state !== 'ready' || Number(row.draft_version) !== scope.draft.version)
                throw new AppError(409, 'Only a ready snapshot matching current sources can be exported');
            const event = randomUUID();
            await db.query("INSERT INTO tender.tsc_events(id,job_id,action,version,reason,created_by_id)VALUES($1,$2,'export_requested',$3,'',$4)", [event, job, row.version, scope.actor.id]);
            await journal(db, {
                id: event, module: 'tender_fms', type: 'tsc_export_request', entity: event, version: 1, action: 'tsc_export_requested', actor: scope.actor.id, project: null, at: new Date().toISOString()
            });
            if (tail[0] === 'evidence') {
                const c = input.coverage;
                let text = `# TSC source evidence\n\nDerived, unapproved reading. Source revisions remain authoritative.\n\nEngine: ${c.version}\nDocuments: ${c.documents}; segments: ${c.segments}; empty segments: ${c.empty_segments}; low-confidence segments: ${c.low_confidence}; blocks: ${c.blocks}; repeated furniture: ${c.suppressed}.\n\n`;
                for (const suppressed of [false, true]) {
                    if (suppressed)
                        text += '# Repeated page furniture retained for fallback\n\n';
                    for (const p of input.passages ?? [])
                        if (!!p.suppressed === suppressed)
                            text += `## ${p.filename} / ${p.locator}\n\nSource: ${p.id}; revision: ${p.revision}; SHA256: ${p.sha256}; method: ${p.method ?? ''}; type: ${p.kind ?? ''}\n\n${p.text}\n\n`;
                }
                return binary(Buffer.from(text), 'TSC-source-evidence.md', 'text/markdown; charset=utf-8');
            }
            const xml = (v: string) => v.replace(/[\x00-\x08\x0b\x0c\x0e-\x1f]/g, '').replace(/[&<>"']/g, c => ({
                '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&apos;'
            }[c]!)), paragraph = (s: string) => `<w:p><w:r><w:t xml:space="preserve">${xml(s)}</w:t></w:r></w:p>`;
            let content = paragraph(input.name) + paragraph('Retained historical checklist · ' + row.mode + ' · ' + row.model);
            for (const q of input.questions) {
                const a = result.answers?.find(a => a.question_id === q.id);
                content += paragraph(q.printed_number + ' ' + q.question) + paragraph(a?.answer ?? '') + (a?.citations ?? []).map(c => paragraph(c.source_id + ': ' + c.quote)).join('');
            }
            const document = `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>${content}<w:sectPr/></w:body></w:document>`, data = Buffer.from(zipSync({ ...Object.fromEntries(Object.entries(template.parts).map(([n, b]) => [n, Buffer.from(b, 'base64')])), 'word/document.xml': strToU8(document) }));
            return binary(data, 'TSC-draft.docx', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document');
        }
        throw new AppError(409, 'Historical local processing is retired. Create a cloud or manual checklist from current documents.');
    });
}
