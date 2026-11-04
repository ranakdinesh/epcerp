import 'server-only';
import { randomUUID } from 'node:crypto';
import type { DB } from '../../db';
import { AppError } from '../../errors';
import { encrypt, decrypt } from './crypto';
import type { Answer, JobRow, Price, Snapshot, Usage } from './types';
import { dailyLimit, estimateMicroINR, reservePaise, tenderLimit } from './pricing';
import type { ProviderReply } from './providers';
export type Task = {
    id: string;
    job_id: string;
    draft_id: string;
    actor_id: string;
    question_id: string | null;
    provider: 'gemini' | 'openai';
    stronger: boolean;
    claim_id: string;
    state: string;
    request_hash: string;
};
export function decodeJob(row: JobRow) { return { snapshot: decrypt<Snapshot>(row.snapshot_cipher, row.snapshot_hash), answers: decrypt<Answer[]>(row.answers_cipher, row.answers_hash) }; }
export async function event(db: DB, row: JobRow, action: string, reason: string, answers: Answer[], actor: string) {
    const payload = encrypt(answers), id = randomUUID();
    await db.query('INSERT INTO tender.tsc_workspace_events(id,job_id,version,action,reason,payload_cipher,payload_hash,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8)', [id, row.id, row.version, action, reason, payload.cipher, payload.hash, actor]);
    await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,visibility,occurred_at)VALUES($1,'tender_fms','tsc_workspace',$2,$3,0,$4,$5,'user',$1,'platform','restricted',clock_timestamp())`, [id, row.id, row.version, action, actor]);
}
export async function getJob(db: DB, draft: string, id: string, forUpdate = false) { const row = (await db.query<JobRow>('SELECT * FROM tender.tsc_workspaces WHERE draft_id=$1 AND id=$2' + (forUpdate ? ' FOR UPDATE' : ''), [draft, id])).rows[0]; if (!row)
    throw new AppError(404, 'TSC draft not found'); return row; }
export async function costs(db: DB, draft: string, job?: string) {
    const rows = (await db.query(`SELECT c.id,c.job_id,c.task_id,c.provider,c.model,c.purpose,c.state,c.error_code,c.reserved_paise,c.pricing,c.manifest,c.created_at,r.id AS response_id,r.estimated_micro_inr,r.cost_basis,r.usage,r.http_status FROM tender.tsc_model_calls c LEFT JOIN tender.tsc_model_responses r ON r.call_id=c.id WHERE c.draft_id=$1 ${job ? 'AND c.job_id=$2' : ''} ORDER BY c.created_at,c.id`, job ? [draft, job] : [draft])).rows;
    let estimated = 0, unknown = 0, reserved = 0;
    for (const r of rows) {
        if (r.estimated_micro_inr == null) {
            unknown++;
            reserved += Number(r.reserved_paise);
        }
        else
            estimated += Number(r.estimated_micro_inr);
    }
    const legacy = Number((await db.query('SELECT coalesce(sum(reserved_paise),0)AS spent FROM tender.tsc_cloud_calls WHERE draft_id=$1', [draft])).rows[0].spent);
    return { estimated_micro_inr: estimated, unknown_requests: unknown, unconfirmed_reserved_paise: reserved, legacy_reserved_paise: legacy, limit_paise: tenderLimit(), requests: rows.map(r => ({ ...r, reserved_paise: Number(r.reserved_paise), estimated_micro_inr: r.estimated_micro_inr == null ? null : Number(r.estimated_micro_inr), manifest: undefined })) };
}
export async function reserve(db: DB, job: JobRow, task: Task, p: Price, key: string, questionIDs: string[], sourceFingerprint: string) {
    // Auth administration is locked by the caller; serialize the shared daily allowance.
    await db.query("SELECT pg_advisory_xact_lock(hashtextextended('tsc-spend:'||(clock_timestamp() AT TIME ZONE 'UTC')::date::text,0))");
    await db.query('SELECT id FROM tender.document_drafts WHERE id=$1 FOR UPDATE', [job.draft_id]);
    const amount = reservePaise(p), old = (await db.query('SELECT id FROM tender.tsc_model_calls WHERE job_id=$1 AND request_key=$2', [job.id, key])).rows[0];
    if (old)
        throw new AppError(409, 'This model request already exists; it will not be sent twice');
    const totals = (await db.query(`SELECT coalesce(sum(CASE WHEN r.estimated_micro_inr IS NULL THEN c.reserved_paise ELSE ceil(r.estimated_micro_inr::numeric/10000)END)FILTER(WHERE c.draft_id=$1),0) AS tender_spent,
 coalesce(sum(CASE WHEN r.estimated_micro_inr IS NULL THEN c.reserved_paise ELSE ceil(r.estimated_micro_inr::numeric/10000)END)FILTER(WHERE c.created_at>=date_trunc('day',clock_timestamp() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),0)AS daily_spent FROM tender.tsc_model_calls c LEFT JOIN tender.tsc_model_responses r ON r.call_id=c.id`, [job.draft_id])).rows[0];
    const legacy = (await db.query(`SELECT coalesce(sum(reserved_paise)FILTER(WHERE draft_id=$1),0)AS tender_spent,coalesce(sum(reserved_paise)FILTER(WHERE created_at>=date_trunc('day',clock_timestamp() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),0)AS daily_spent FROM tender.tsc_cloud_calls`, [job.draft_id])).rows[0];
    if (Number(totals.tender_spent) + Number(legacy.tender_spent) + amount > tenderLimit())
        throw new AppError(409, 'Tender processing allowance reached; use manual writing or ask an administrator');
    if (Number(totals.daily_spent) + Number(legacy.daily_spent) + amount > dailyLimit())
        throw new AppError(429, 'Daily processing allowance reached; manual writing remains available');
    const id = randomUUID();
    await db.query(`INSERT INTO tender.tsc_model_calls(id,job_id,draft_id,actor_id,task_id,request_key,provider,model,purpose,manifest,pricing,reserved_paise)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, [id, job.id, job.draft_id, task.actor_id, task.id, key, p.provider, p.model, task.question_id ? 'question_proposal' : 'checklist_generation', { prompt_version: 'tsc-originals-v2', source_fingerprint: sourceFingerprint, questions: questionIDs, sources: decodeJob(job).snapshot.sources.map(s => ({ id: s.id, sha256: s.sha256 })), references: 'model_reported_unverified' }, p, amount]);
    return id;
}
export async function recordReply(db: DB, call: string, job: JobRow, reply: ProviderReply, answers: Answer[], p: Price) {
    const raw = encrypt(reply.raw), parsed = encrypt(answers);
    let estimated: number | null = null, usage: Usage | null = reply.usage;
    if (usage)
        try {
            estimated = estimateMicroINR(p, usage);
        }
        catch {
            usage = null;
        }
    // A conclusive pre-generation upload failure incurs no generation tokens.
    if (reply.status === 0 && reply.error !== 'provider_outcome_uncertain')
        estimated = 0;
    const basis = p.tier === 'free' && usage ? 'free_tier' : usage ? 'reported_usage' : 'usage_unavailable';
    await db.query(`INSERT INTO tender.tsc_model_responses(id,call_id,job_id,draft_id,http_status,provider_request_id,raw_cipher,raw_hash,parsed_cipher,parsed_hash,usage,estimated_micro_inr,cost_basis)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, [randomUUID(), call, job.id, job.draft_id, reply.status, reply.request_id, raw.cipher, raw.hash, parsed.cipher, parsed.hash, usage ?? {}, estimated, basis]);
    await db.query("UPDATE tender.tsc_model_calls SET state=$2,error_code=$3,updated_at=clock_timestamp()WHERE id=$1 AND state IN('reserved','sent')", [call, reply.error === 'provider_outcome_uncertain' ? 'uncertain' : reply.error ? 'failed' : 'completed', reply.error]);
}
