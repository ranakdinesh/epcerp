import { randomUUID, createHash } from 'node:crypto';
import type { DB } from '../../db';
import type { Actor } from './context';
import type { Budget } from './types';
import { project } from '../../purchasing/core/access';
import { projectAccess } from '../../purchasing/core/project-access';
import { assignmentIdentity } from '../../auth/access';
import { journal } from '../../purchasing/core/journal';
import { directorDecisionNotice } from '../../inbox/purchasing-notices';
import { goJSON } from '../../master/catalogue/serialization';
import { canonicalData } from './repository';
import { BudgetError, invalid } from './validation';
import { now } from '../../purchasing/core/time';
export const fingerprint = (d: Budget['data']) => createHash('sha256').update(goJSON(canonicalData(d))).digest('hex');
export async function approve(db: DB, a: Actor, b: Budget, reason: string) {
    const live = await assignmentIdentity(db, a.id);
    if (!live?.active)
        throw new BudgetError(401, 'Invalid or expired credentials');
    const p = await project(db, b.project_id, true);
    for (const code of ['projects.budgets.read', 'projects.budgets.approve']) {
        if (!a.allows(code) || !live.allows(code))
            throw new BudgetError(403, 'Financial decision permission or project scope denied');
        await projectAccess(db, a, p, code, code.endsWith('.approve'));
    }
    if (!/^[A-Z]{3}(?![\s\S])/.test(b.data.currency))
        invalid('currency', 'Use an ISO currency code');
    const id = randomUUID(), at = now();
    try {
        await db.query('INSERT INTO finance_budget_decisions(id,project_id,source_version,source_revision,amount,currency,source_fingerprint,reason,is_active,created_at,updated_at,created_by_id,updated_by_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,true,$9,$9,$10,$10)', [id, b.project_id, b.version, b.revision, b.data.total, b.data.currency, fingerprint(b.data), reason, at, a.id]);
    }
    catch (e) {
        if ((e as {
            code?: string;
        }).code === '23505')
            throw new BudgetError(409, 'Financial decision already recorded');
        throw e;
    }
    await journal(db, {
        id, module: 'finance_approvals', type: 'financial_decision', entity: id, version: 1, action: 'approved', actor: a.id, project: b.project_id, at
    });
    await directorDecisionNotice(db, id);
    return id;
}
