import 'server-only';
import { randomUUID } from 'node:crypto';
import type { DB } from '../../db';
import { AppError } from '../../errors';
import {confirmedRejection} from './billing';
import { encrypt, decrypt } from './crypto';
import type { Answer, JobRow, Price, Snapshot, Usage, Provider } from './types';
import { dailyLimit, estimateMicroINR, reservePaise, tenderLimit } from './pricing';
import type { ProviderReply } from './providers';
export type Task = {
    id: string;
    job_id: string;
    draft_id: string;
    actor_id: string;
    question_id: string | null;
    provider: Provider;
    stronger: boolean;
    claim_id: string;
    state: string;
    request_hash: string;
};
export function decodeJob(row: JobRow) { return { snapshot: decrypt<Snapshot>(row.snapshot_cipher, row.snapshot_hash), answers: decrypt<Answer[]>(row.answers_cipher, row.answers_hash) }; }
export async function event(db: DB, row: JobRow, action: string, reason: string, answers: Answer[], actor: string, retainedDocument?:{name:string;sha256:string;base64:string;mapping:{row_id:number;question_id:string}[]}) {
    const payload = encrypt(retainedDocument?{answers,document:retainedDocument}:answers), id = randomUUID();
    await db.query('INSERT INTO tender.tsc_workspace_events(id,job_id,version,action,reason,payload_cipher,payload_hash,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8)', [id, row.id, row.version, action, reason, payload.cipher, payload.hash, actor]);
    await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,visibility,occurred_at)VALUES($1,'tender_fms','tsc_workspace',$2,$3,0,$4,$5,'user',$1,'platform','restricted',clock_timestamp())`, [id, row.id, row.version, action, actor]);
}
export async function getJob(db: DB, draft: string, id: string, forUpdate = false) { const row = (await db.query<JobRow>('SELECT * FROM tender.tsc_workspaces WHERE draft_id=$1 AND id=$2' + (forUpdate ? ' FOR UPDATE' : ''), [draft, id])).rows[0]; if (!row)
    throw new AppError(404, 'TSC draft not found'); return row; }
export async function costs(db: DB, draft: string, job?: string) {
    const rows = (await db.query(`SELECT c.id,c.job_id,c.task_id,c.provider,c.model,c.purpose,c.state,c.error_code,c.reserved_paise,c.pricing,c.manifest,c.created_at,r.id AS response_id,r.estimated_micro_inr,r.cost_basis,r.usage,r.http_status,CASE WHEN r.http_status IN(429,503) THEN r.raw_cipher END AS raw_cipher,r.raw_hash FROM tender.tsc_model_calls c LEFT JOIN tender.tsc_model_responses r ON r.call_id=c.id WHERE c.draft_id=$1 ${job ? 'AND c.job_id=$2' : ''} ORDER BY c.created_at,c.id`, job ? [draft, job] : [draft])).rows;
    let estimated = 0, unknown = 0, reserved = 0;
    for (const r of rows) {
        if(r.estimated_micro_inr==null&&r.raw_cipher&&confirmedRejection(r.provider,r.http_status,decrypt<string>(r.raw_cipher,r.raw_hash))){r.estimated_micro_inr=0;r.cost_basis='provider_rejection';}
        if (r.estimated_micro_inr == null) {
            unknown++;
            reserved += Number(r.reserved_paise);
        }
        else
            estimated += Number(r.estimated_micro_inr);
    }
    const legacy = Number((await db.query('SELECT coalesce(sum(reserved_paise),0)AS spent FROM tender.tsc_cloud_calls WHERE draft_id=$1', [draft])).rows[0].spent);
    return { estimated_micro_inr: estimated, unknown_requests: unknown, unconfirmed_reserved_paise: reserved, legacy_reserved_paise: legacy, limit_paise: tenderLimit(), requests: rows.map(row => {const r={...row};delete r.raw_cipher;delete r.raw_hash;return ({ ...r, reserved_paise: Number(r.reserved_paise), estimated_micro_inr: r.estimated_micro_inr == null ? null : Number(r.estimated_micro_inr), manifest: undefined })}) };
}
export async function reserve(db: DB, job: JobRow, task: Task, p: Price, key: string, questionIDs: string[], sourceFingerprint: string) {
    // Auth administration is locked by the caller; serialize the shared daily allowance.
    await db.query("SELECT pg_advisory_xact_lock(hashtextextended('tsc-spend:'||(clock_timestamp() AT TIME ZONE 'UTC')::date::text,0))");
    await db.query('SELECT id FROM tender.document_drafts WHERE id=$1 FOR UPDATE', [job.draft_id]);
    const amount = reservePaise(p), old = (await db.query('SELECT id FROM tender.tsc_model_calls WHERE job_id=$1 AND request_key=$2', [job.id, key])).rows[0];
    if (old)
        throw new AppError(409, 'This model request already exists; it will not be sent twice');
    const calls=(await db.query(`SELECT c.draft_id,c.provider,c.reserved_paise,c.created_at>=date_trunc('day',clock_timestamp() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS today,r.estimated_micro_inr,r.http_status,CASE WHEN r.http_status IN(429,503) THEN r.raw_cipher END AS raw_cipher,r.raw_hash FROM tender.tsc_model_calls c LEFT JOIN tender.tsc_model_responses r ON r.call_id=c.id WHERE c.draft_id=$1 OR c.created_at>=date_trunc('day',clock_timestamp() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'`,[job.draft_id])).rows;
    const totals={tender_spent:0,daily_spent:0};
    for(const c of calls){const rejected=c.estimated_micro_inr==null&&c.raw_cipher&&confirmedRejection(c.provider,c.http_status,decrypt<string>(c.raw_cipher,c.raw_hash)),spent=c.estimated_micro_inr!=null?Math.ceil(Number(c.estimated_micro_inr)/10000):rejected?0:Number(c.reserved_paise);if(c.draft_id===job.draft_id)totals.tender_spent+=spent;if(c.today)totals.daily_spent+=spent;}
    const legacy = (await db.query(`SELECT coalesce(sum(reserved_paise)FILTER(WHERE draft_id=$1),0)AS tender_spent,coalesce(sum(reserved_paise)FILTER(WHERE created_at>=date_trunc('day',clock_timestamp() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),0)AS daily_spent FROM tender.tsc_cloud_calls`, [job.draft_id])).rows[0];
    if (Number(totals.tender_spent) + Number(legacy.tender_spent) + amount > tenderLimit())
        throw new AppError(409, 'Tender processing allowance reached; use manual writing or ask an administrator');
    if (Number(totals.daily_spent) + Number(legacy.daily_spent) + amount > dailyLimit())
        throw new AppError(429, 'Daily processing allowance reached; manual writing remains available');
    const id = randomUUID();
    await db.query(`INSERT INTO tender.tsc_model_calls(id,job_id,draft_id,actor_id,task_id,request_key,provider,model,purpose,manifest,pricing,reserved_paise)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, [id, job.id, job.draft_id, task.actor_id, task.id, key, p.provider, p.model, task.question_id ? 'question_proposal' : 'checklist_generation', { prompt_version: p.input_mode?'tsc-source-checked-v4':'tsc-originals-v2', source_fingerprint: sourceFingerprint, questions: questionIDs, sources: decodeJob(job).snapshot.sources.map(s => ({ id: s.id, sha256: s.sha256 })), references: 'model_reported_unverified' }, p, amount]);
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
    const rejected=!usage&&confirmedRejection(p.provider,reply.status,reply.raw);if(rejected)estimated=0;
    const basis = p.tier === 'free' && usage ? 'free_tier' : usage ? 'reported_usage' : 'usage_unavailable';
    await db.query(`INSERT INTO tender.tsc_model_responses(id,call_id,job_id,draft_id,http_status,provider_request_id,raw_cipher,raw_hash,parsed_cipher,parsed_hash,usage,estimated_micro_inr,cost_basis,input_evidence)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, [randomUUID(), call, job.id, job.draft_id, reply.status, reply.request_id, raw.cipher, raw.hash, parsed.cipher, parsed.hash, usage ?? {}, estimated, basis,{...reply.evidence,...(rejected?{billing_resolution:'structured_google_rejection_before_generation'}:{})}]);
    await db.query("UPDATE tender.tsc_model_calls SET state=$2,error_code=$3,updated_at=clock_timestamp()WHERE id=$1 AND state IN('reserved','sent')", [call, reply.error === 'provider_outcome_uncertain' ? 'uncertain' : reply.error ? 'failed' : 'completed', reply.error]);
}
