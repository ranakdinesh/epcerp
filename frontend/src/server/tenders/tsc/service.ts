import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction } from '../../db';
import { AppError } from '../../errors';
import { authorizeTender, referenceDocument, sourceFingerprint, tenderSources } from '../documents';
import { digest, encrypt, decrypt } from './crypto';
import { configured } from './providers';
import { price, reservePaise } from './pricing';
import { blank, flag, id, integer, invalid, managerAnswer, mode, object, sourceSelection, str } from './validation';
import { costs, decodeJob, event, getJob } from './repository';
import type { Answer, JobRow, Question, Snapshot } from './types';
export function options(token: string, draft: string) { return transaction(async (db) => { const { actor } = await authorizeTender(db, token, draft), sources = await tenderSources(db, draft, actor.id), formats = (await db.query(`SELECT f.id,f.name,f.description,f.version,f.purpose,count(q.*)::int AS question_count FROM tender.tsc_formats f JOIN tender.tsc_format_questions q ON q.format_id=f.id WHERE f.is_active GROUP BY f.id ORDER BY f.name_key LIMIT 500`)).rows; return { engine: 'nextjs', formats: formats.map(f => ({ ...f, version: Number(f.version) })), sources: sources.map(s => ({ ...s, reference_document: referenceDocument(s.path) })), providers: ['gemini', 'openai'].map(p => { const provider = p as 'gemini' | 'openai', rate = price(provider); return { mode: p, model: rate.model, available: configured(provider), reservation_paise: reservePaise(rate), pricing: rate }; }), cost: await costs(db, draft) }; }); }
export async function create(token: string, draft: string, key: string, value: unknown) {
    id(key);
    const r = object(value, ['expected_version', 'format_id', 'format_version', 'mode', 'source_ids', 'auto_escalate', 'original_documents_confirmed']), requested = mode(r.mode), version = integer(r.expected_version, 1), formatID = id(r.format_id), formatVersion = integer(r.format_version, 1), escalate = r.auto_escalate == null ? false : flag(r.auto_escalate);
    if (requested !== 'manual' && r.original_documents_confirmed !== true)
        invalid('Confirm the selected documents and processing allowance before generation');
    return transaction(async (db) => {
        const { actor, draft: d } = await authorizeTender(db, token, draft, 'write');
        const requestHash = digest(JSON.stringify({ version, formatID, formatVersion, requested, sources: r.source_ids, escalate })), previous = (await db.query<JobRow>('SELECT * FROM tender.tsc_workspaces WHERE draft_id=$1 AND actor_id=$2 AND operation_key=$3', [draft, actor.id, key])).rows[0];
        if (previous) {
            if (previous.request_hash !== requestHash)
                throw new AppError(409, 'Operation key belongs to another request');
            return jobView(db, previous, d.version);
        }
        if (requested !== 'manual') {
            if (!actor.allows('tenders.tsc.originals.write'))
                throw new AppError(403, 'Permission to send original documents is required');
            if (!configured(requested))
                throw new AppError(503, 'Selected provider is not approved and configured; manual writing is available');
        }
        if (d.version !== version)
            throw new AppError(409, 'Tender sources changed; refresh before generating');
        const retained = Number((await db.query('SELECT (SELECT count(*)FROM tender.tsc_jobs WHERE draft_id=$1)+(SELECT count(*)FROM tender.tsc_workspaces WHERE draft_id=$1)AS count', [draft])).rows[0].count);
        if (retained >= 50)
            invalid('This tender has reached its 50-generation retention limit');
        const format = (await db.query('SELECT * FROM tender.tsc_formats WHERE id=$1 AND is_active FOR SHARE', [formatID])).rows[0];
        if (!format || Number(format.version) !== formatVersion)
            throw new AppError(409, 'TSC format changed; refresh before generating');
        const questions = (await db.query<Question>('SELECT id,section,printed_number,question,manual_review FROM tender.tsc_format_questions WHERE format_id=$1 ORDER BY position', [formatID])).rows;
        if (!questions.length)
            invalid('Select a format with questions');
        const sources = sourceSelection(r.source_ids ?? [], await tenderSources(db, draft, actor.id), requested);
        if (sources.some(s => referenceDocument(s.path)))
            invalid('Completed checklist/reference documents cannot be used as generation evidence');
        const snapshot: Snapshot = { schema: 'tsc-cloud-v2', draft_id: draft, draft_version: version, name: d.name, format_id: formatID, format_version: formatVersion, format_name: format.name, questions, sources, mode: requested, auto_escalate: requested === 'gemini' && escalate, source_fingerprint: sourceFingerprint(sources) }, input = encrypt(snapshot), answers = questions.map(blank), result = encrypt(answers), jobID = randomUUID();
        const row = (await db.query<JobRow>(`INSERT INTO tender.tsc_workspaces(id,draft_id,draft_version,actor_id,operation_key,request_hash,mode,state,snapshot_cipher,snapshot_hash,answers_cipher,answers_hash)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)RETURNING *`, [jobID, draft, version, actor.id, key, requestHash, requested, requested === 'manual' ? 'ready' : 'queued', input.cipher, input.hash, result.cipher, result.hash])).rows[0];
        if (requested !== 'manual')
            await db.query('INSERT INTO tender.tsc_workspace_tasks(id,job_id,draft_id,actor_id,operation_key,request_hash,provider)VALUES($1,$2,$3,$4,$5,$6,$7)', [randomUUID(), jobID, draft, actor.id, randomUUID(), requestHash, requested]);
        await event(db, row, 'created', 'Original model responses are retained separately from working answers', answers, actor.id);
        return jobView(db, row, version);
    });
}
async function jobView(db: Parameters<typeof costs>[0], row: JobRow, version: number) { const decoded = decodeJob(row), tasks = (await db.query('SELECT id,question_id,provider,stronger,state,error_code,created_at FROM tender.tsc_workspace_tasks WHERE job_id=$1 ORDER BY created_at DESC LIMIT 20', [row.id])).rows; return { id: row.id, draft_id: row.draft_id, draft_version: Number(row.draft_version), version: Number(row.version), state: row.state, mode: row.mode, error_code: row.error_code, created_at: row.created_at, stale: Number(row.draft_version) !== version, ...decoded, cost: await costs(db, row.draft_id, row.id), tasks }; }
export function get(token: string, draft: string, job: string) { return transaction(async (db) => { const { draft: d } = await authorizeTender(db, token, draft); return jobView(db, await getJob(db, draft, job), d.version); }); }
export function list(token: string, draft: string) { return transaction(async (db) => { await authorizeTender(db, token, draft); return (await db.query("SELECT * FROM(SELECT id,mode,state,version,draft_version,created_at,error_code,'nextjs'::text AS engine FROM tender.tsc_workspaces WHERE draft_id=$1 UNION ALL SELECT id,mode,state,version,draft_version,created_at,error_code,'go'::text AS engine FROM tender.tsc_jobs WHERE draft_id=$1)jobs ORDER BY created_at DESC LIMIT 50", [draft])).rows.map(r => ({ ...r, version: Number(r.version), draft_version: Number(r.draft_version) })); }); }
export function edit(token: string, draft: string, job: string, question: string, value: unknown) {
    const r = object(value, ['expected_version', 'answer', 'reason']), version = integer(r.expected_version, 1), reason = str(r.reason, 1000);
    if (reason.trim().length < 3)
        invalid('Provide a correction reason');
    return transaction(async (db) => {
        const { actor, draft: d } = await authorizeTender(db, token, draft, 'write'), row = await getJob(db, draft, job, true), { snapshot, answers } = decodeJob(row);
        if (row.state !== 'ready' || Number(row.draft_version) !== d.version || Number(row.version) !== version)
            throw new AppError(409, 'TSC draft changed; keep your text and review the latest saved version');
        const next = managerAnswer(r.answer, snapshot);
        if (next.question_id !== question)
            invalid('Question identity does not match');
        const index = answers.findIndex(a => a.question_id === question);
        if (index < 0)
            invalid('Unknown question');
        const old = answers[index];
        if (old.accepted && (old.answer !== next.answer || JSON.stringify(old.citations) !== JSON.stringify(next.citations) || old.status !== next.status))
            throw new AppError(409, 'Reopen the reviewed answer before changing its wording');
        if (old.answer === next.answer && old.status === next.status && JSON.stringify(old.citations) === JSON.stringify(next.citations))
            next.origin = old.origin;
        answers[index] = next;
        const payload = encrypt(answers), saved = (await db.query<JobRow>('UPDATE tender.tsc_workspaces SET answers_cipher=$2,answers_hash=$3,version=version+1,updated_at=clock_timestamp()WHERE id=$1 RETURNING *', [row.id, payload.cipher, payload.hash])).rows[0];
        await event(db, saved, 'manager_edit', reason, answers, actor.id);
        return jobView(db, saved, d.version);
    });
}
export function requestProposal(token: string, draft: string, job: string, question: string, key: string, value: unknown) {
    id(key);
    const r = object(value, ['expected_version', 'provider', 'stronger', 'original_documents_confirmed']), provider = mode(r.provider), stronger = flag(r.stronger ?? false), version = integer(r.expected_version, 1);
    if (provider === 'manual')
        invalid('Choose a model provider');
    if (r.original_documents_confirmed !== true)
        invalid('Confirm the model request allowance');
    return transaction(async (db) => {
        const { actor, draft: d } = await authorizeTender(db, token, draft, 'originals.write');
        const row = await getJob(db, draft, job, true), { snapshot, answers } = decodeJob(row), requestHash = digest(JSON.stringify({ question, provider, stronger, version })), old = (await db.query('SELECT * FROM tender.tsc_workspace_tasks WHERE job_id=$1 AND operation_key=$2', [job, key])).rows[0];
        if (old) {
            if (old.request_hash !== requestHash)
                throw new AppError(409, 'Operation key belongs to another question request');
            return { task_id: old.id, state: old.state };
        }
        if (row.state !== 'ready' || Number(row.version) !== version || Number(row.draft_version) !== d.version)
            throw new AppError(409, 'Refresh the current TSC draft before requesting a proposal');
        const q = snapshot.questions.find(q => q.id === question);
        if (!q || q.manual_review)
            invalid('This question requires manual writing');
        if (answers.find(a => a.question_id === question)?.accepted)
            invalid('Reopen the reviewed answer before requesting another proposal');
        if (!snapshot.sources.length)
            invalid('This manual draft has no selected source documents; create a cloud draft with sources');
        if (!configured(provider))
            throw new AppError(503, 'Provider is not approved and configured');
        const task = randomUUID();
        await db.query('INSERT INTO tender.tsc_workspace_tasks(id,job_id,draft_id,actor_id,operation_key,request_hash,question_id,provider,stronger)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)', [task, job, draft, actor.id, key, requestHash, question, provider, stronger]);
        return { task_id: task, state: 'queued' };
    });
}
export function originals(token: string, draft: string, job: string, question: string) { return transaction(async (db) => { await authorizeTender(db, token, draft); const row = await getJob(db, draft, job); if (!decodeJob(row).snapshot.questions.some(q => q.id === question))
    invalid('Unknown question'); const responses = (await db.query('SELECT r.*,c.provider,c.model FROM tender.tsc_model_responses r JOIN tender.tsc_model_calls c ON c.id=r.call_id WHERE r.job_id=$1 AND r.draft_id=$2 ORDER BY r.created_at,r.id', [job, draft])).rows; return responses.map(r => ({ id: r.id, provider: r.provider, model: r.model, created_at: r.created_at, estimated_micro_inr: r.estimated_micro_inr == null ? null : Number(r.estimated_micro_inr), answer: decrypt<Answer[]>(r.parsed_cipher, r.parsed_hash).find(a => a.question_id === question) })).filter(r => r.answer); }); }
export function history(token: string, draft: string, job: string) { return transaction(async (db) => { await authorizeTender(db, token, draft); await getJob(db, draft, job); return (await db.query('SELECT id,version,action,reason,created_at,created_by_id FROM tender.tsc_workspace_events WHERE job_id=$1 ORDER BY version DESC LIMIT 200', [job])).rows.map(r => ({ ...r, version: Number(r.version) })); }); }
export function responseRaw(token: string, draft: string, job: string, response: string) { return transaction(async (db) => { await authorizeTender(db, token, draft); await getJob(db, draft, job); const r = (await db.query('SELECT raw_cipher,raw_hash FROM tender.tsc_model_responses WHERE id=$1 AND job_id=$2 AND draft_id=$3', [response, job, draft])).rows[0]; if (!r)
    throw new AppError(404, 'Model response not found'); return decrypt<string>(r.raw_cipher, r.raw_hash); }); }
export function exportSnapshot(token: string, draft: string, job: string) { return transaction(async (db) => { const { draft: d } = await authorizeTender(db, token, draft, 'export'), row = await getJob(db, draft, job); if (row.state !== 'ready' || Number(row.draft_version) !== d.version)
    throw new AppError(409, 'Export a ready draft matching current source versions'); return jobView(db, row, d.version); }); }
export function isLegacy(token: string, draft: string, job: string) { return transaction(async (db) => { await authorizeTender(db, token, draft); return !!(await db.query('SELECT id FROM tender.tsc_jobs WHERE draft_id=$1 AND id=$2', [draft, job])).rows[0]; }); }
