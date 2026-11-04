import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, pool, actor, lock } from '../../db';
import { AppError } from '../../errors';
import { authorizeWorker, originalDocuments } from '../documents';
import { encrypt } from './crypto';
import { configured, ProviderSession } from './providers';
import { price } from './pricing';
import { providerAnswers, blank } from './validation';
import { decodeJob, event, getJob, recordReply, reserve, type Task } from './repository';
import type { Answer, JobRow, Question } from './types';
const failure = (e: unknown) => e instanceof AppError ? e.status === 403 ? 'permission_changed' : e.status === 409 ? 'source_or_budget_changed' : e.status === 429 ? 'daily_allowance_reached' : 'processing_unavailable' : 'processing_unavailable';
async function fence(db: Parameters<typeof getJob>[0], task: Task) { const current = (await db.query("SELECT id FROM tender.tsc_workspace_tasks WHERE id=$1 AND state='processing' AND claim_id=$2 AND lease_until>clock_timestamp() FOR UPDATE", [task.id, task.claim_id])).rows[0]; if (!current)
    throw new AppError(409, 'Processing claim expired'); const job = await getJob(db, task.draft_id, task.job_id, true), draft = await authorizeWorker(db, task.actor_id, task.draft_id); if (Number(draft.version) !== Number(job.draft_version))
    throw new AppError(409, 'Tender sources changed during processing'); if (process.env.EPCERP_TSC_OWNER !== 'nextjs' || !configured(task.provider))
    throw new AppError(403, 'Provider execution is disabled'); await actor(db, task.actor_id); return job; }
async function complete(task: Task, initial: JobRow, answers: Answer[], error: string) {
    return transaction(async (db) => {
        await lock(db, 'auth.administration');
        const current = (await db.query("SELECT id FROM tender.tsc_workspace_tasks WHERE id=$1 AND state='processing' AND claim_id=$2 AND lease_until>clock_timestamp() FOR UPDATE", [task.id, task.claim_id])).rows[0];
        if (!current)
            return;
        const row = await getJob(db, task.draft_id, task.job_id, true), state = task.question_id ? row.state : 'ready';
        let result = decodeJob(row).answers;
        if (!task.question_id)
            try {
                await authorizeWorker(db, task.actor_id, task.draft_id);
                const draft = (await db.query('SELECT version FROM tender.document_drafts WHERE id=$1', [task.draft_id])).rows[0];
                if (Number(draft.version) !== Number(initial.draft_version))
                    throw Error('source_changed');
                result = answers;
            }
            catch {
                error = 'permission_or_source_changed';
            }
        const sealed = encrypt(result), saved = (await db.query<JobRow>('UPDATE tender.tsc_workspaces SET answers_cipher=$2,answers_hash=$3,state=$4,claim_id=NULL,lease_until=NULL,error_code=$5,version=version+1,updated_at=clock_timestamp()WHERE id=$1 RETURNING *', [row.id, sealed.cipher, sealed.hash, state, error])).rows[0];
        await db.query("UPDATE tender.tsc_workspace_tasks SET state=$3,error_code=$4,claim_id=NULL,lease_until=NULL,updated_at=clock_timestamp()WHERE id=$1 AND claim_id=$2", [task.id, task.claim_id, error ? 'failed' : 'ready', error]);
        await event(db, saved, task.question_id ? 'proposal_received' : 'generation_completed', error || 'Model responses retained; manager answers remain independently editable', result, task.actor_id);
    });
}
export async function workOnce(sessionFactory = () => new ProviderSession()): Promise<boolean> {
    const task = await transaction(async (db) => {
        // An expired paid attempt is never resent. Retain its allowance and expose manual recovery.
        const expired = (await db.query<Task>("SELECT * FROM tender.tsc_workspace_tasks WHERE state='processing' AND lease_until<clock_timestamp() FOR UPDATE SKIP LOCKED LIMIT 1")).rows[0];
        if (expired) {
            await db.query("UPDATE tender.tsc_model_calls SET state='uncertain',error_code='worker_interrupted',updated_at=clock_timestamp()WHERE task_id=$1 AND state IN('reserved','sent')", [expired.id]);
            await db.query("UPDATE tender.tsc_workspace_tasks SET state='failed',claim_id=NULL,lease_until=NULL,error_code='worker_interrupted',updated_at=clock_timestamp()WHERE id=$1", [expired.id]);
            const row = await getJob(db, expired.draft_id, expired.job_id, true), answers = decodeJob(row).answers, saved = (await db.query<JobRow>("UPDATE tender.tsc_workspaces SET state='ready',claim_id=NULL,lease_until=NULL,error_code='worker_interrupted',version=version+1,updated_at=clock_timestamp()WHERE id=$1 RETURNING *", [row.id])).rows[0];
            await event(db, saved, 'processing_interrupted', 'No automatic paid retry; retained responses and manual writing remain available', answers, expired.actor_id);
        }
        return (await db.query<Task>(`WITH candidate AS(SELECT id FROM tender.tsc_workspace_tasks WHERE state='queued' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1)
 UPDATE tender.tsc_workspace_tasks t SET state='processing',claim_id=$1,lease_until=clock_timestamp()+interval '5 minutes',updated_at=clock_timestamp()FROM candidate c WHERE t.id=c.id RETURNING t.*`, [randomUUID()])).rows[0] ?? null;
    });
    if (!task)
        return false;
    let initial: JobRow | undefined, answers: Answer[] = [], error = '';
    const session = sessionFactory(), heartbeat = setInterval(() => { void pool().query("UPDATE tender.tsc_workspace_tasks SET lease_until=clock_timestamp()+interval '5 minutes' WHERE id=$1 AND state='processing' AND claim_id=$2 AND lease_until>clock_timestamp()", [task.id, task.claim_id]).catch(() => { }); }, 30000);
    heartbeat.unref();
    try {
        const loaded = await transaction(async (db) => { await lock(db, 'auth.administration'); const job = await fence(db, task); if (!task.question_id)
            await db.query("UPDATE tender.tsc_workspaces SET state='processing',claim_id=$2,lease_until=clock_timestamp()+interval '15 minutes' WHERE id=$1", [job.id, task.claim_id]); const { snapshot, answers } = decodeJob(job); if (task.question_id && answers.find(a => a.question_id === task.question_id)?.accepted)
            throw new AppError(409, 'Answer was reviewed before processing started'); return { job, snapshot, documents: await originalDocuments(db, snapshot, task.actor_id) }; });
        initial = loaded.job;
        answers = loaded.snapshot.questions.map(blank);
        const questions = loaded.snapshot.questions.filter(q => !q.manual_review && (!task.question_id || q.id === task.question_id));
        async function run(batch: Question[], stronger: boolean, key: string) {
            if (!batch.length)
                return [];
            const p = price(task!.provider, stronger), call = await transaction(async (db) => { await lock(db, 'auth.administration'); const job = await fence(db, task!); return reserve(db, job, task!, p, task!.id + ':' + key, batch.map(q => q.id), loaded.snapshot.source_fingerprint); });
            const reply = await session.propose(p, batch, loaded.documents, async () => { await transaction(async (db) => { await lock(db, 'auth.administration'); await fence(db, task!); const r = await db.query("UPDATE tender.tsc_model_calls SET state='sent',updated_at=clock_timestamp()WHERE id=$1 AND state='reserved' RETURNING id", [call]); if (!r.rows.length)
                throw new AppError(409, 'Request already started'); }); });
            let parsed: Answer[] = [];
            if (!reply.error)
                try {
                    parsed = providerAnswers(reply.content, batch, loaded.snapshot.sources, task!.provider);
                }
                catch {
                    reply.error = 'invalid_provider_answers';
                }
            // Always retain a received response, including invalid/failed output, before deciding whether to use it.
            await transaction(db => recordReply(db, call, loaded.job, reply, parsed, p));
            if (reply.error) {
                error = reply.error;
                return [];
            }
            return parsed;
        }
        for (let start = 0; start < questions.length; start += 80) {
            const batch = questions.slice(start, start + 80), result = await run(batch, task.stronger, 'initial:' + start);
            for (const a of result) {
                const i = answers.findIndex(x => x.question_id === a.question_id);
                if (i >= 0)
                    answers[i] = a;
            }
            if (error)
                break;
        }
        if (!error && !task.question_id && loaded.snapshot.auto_escalate && task.provider === 'gemini') {
            const unresolved = loaded.snapshot.questions.filter(q => !q.manual_review && answers.some(a => a.question_id === q.id && a.status !== 'answered')).slice(0, 20);
            if (unresolved.length) {
                const result = await run(unresolved, true, 'stronger:0');
                for (const a of result) {
                    const i = answers.findIndex(x => x.question_id === a.question_id);
                    if (i >= 0)
                        answers[i] = a;
                }
            }
        }
    }
    catch (e) {
        error = failure(e);
    }
    finally {
        await session.close();
        clearInterval(heartbeat);
    }
    if (!initial) {
        initial = await transaction(db => getJob(db, task.draft_id, task.job_id));
        answers = decodeJob(initial).answers;
    }
    await complete(task, initial, answers, error);
    return true;
}
const globalWorker = globalThis as typeof globalThis & {
    tscCloudWorkerStarted?: boolean;
};
export function startWorker() { if (globalWorker.tscCloudWorkerStarted || process.env.EPCERP_TSC_OWNER !== 'nextjs')
    return; globalWorker.tscCloudWorkerStarted = true; let running = false; const tick = async () => { if (running)
    return; running = true; try {
    await workOnce();
}
catch {
    console.error('TSC work is unavailable; retained job status will show recovery');
}
finally {
    running = false;
} }; const timer = setInterval(() => void tick(), 2000); timer.unref(); }
