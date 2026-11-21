import { randomUUID } from 'node:crypto';
import type { DB } from '../../db';
import type { Actor } from '../core/access';
import { project } from '../core/access';
import { projectAccess } from '../core/project-access';
import { assignmentIdentity } from '../../auth/access';
import { PurchaseError, invalid } from '../core/validation';
import { journal } from '../core/journal';
import { directorDecisionNotice } from '../../inbox/purchasing-notices';
import type { ApprovalRecord, Attempt } from './types';
import { fingerprint } from './fingerprint';
import { money } from './policy';
import { now } from '../core/time';
export async function decide(db: DB, a: Actor, r: ApprovalRecord, attempt: Attempt, reason: string) {
    const live = await assignmentIdentity(db, a.id);
    if (!live?.active)
        throw new PurchaseError(401, 'Invalid or expired credentials');
    const p = await project(db, r.project_id, true);
    for (const code of ['accounts.quotation_approvals.view', 'accounts.quotation_approvals.read', 'accounts.quotation_approvals.write']) {
        if (!a.allows(code) || !live.allows(code))
            throw new PurchaseError(403, 'Financial decision permission or project scope denied');
        await projectAccess(db, a, p, code, code.endsWith('.write'));
    }
    if (attempt.submitted_by_id === a.id)
        throw new PurchaseError(403, 'Financial decision permission or project scope denied');
    money(attempt.total);
    if (!/^[A-Z]{3}$/.test(attempt.currency))
        invalid('currency', 'Use an ISO currency code');
    if (r.version < 1 || !['approved', 'rejected'].includes(attempt.state))
        invalid('approval', 'Invalid financial decision');
    const id = randomUUID(), at = now();
    try {
        await db.query(`INSERT INTO finance_quotation_decisions(id,project_id,bo_id,attempt_id,source_version,submitted_by_id,outcome,amount,currency,source_fingerprint,reason,is_active,created_at,updated_at,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,true,$12,$12,$13,$13)`, [id, r.project_id, r.bo_id, attempt.id, r.version, attempt.submitted_by_id, attempt.state, attempt.total, attempt.currency, fingerprint(attempt), reason, at, a.id]);
    }
    catch (e) {
        if ((e as {
            code?: string;
        }).code === '23505')
            throw new PurchaseError(409, 'Financial decision already recorded');
        throw e;
    }
    await journal(db, { id, module: 'finance_approvals', type: 'financial_decision', entity: id, version: 1, action: attempt.state, actor: a.id, project: r.project_id, at });
    await directorDecisionNotice(db, id);
    return id;
}
