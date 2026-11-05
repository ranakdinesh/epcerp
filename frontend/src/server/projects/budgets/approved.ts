import type { DB } from '../../db';
import { project } from '../../purchasing/core/access';
import type { Approved } from './types';
export type ApprovedSource = {
    revision: number;
    currency: string;
    decision_id: string;
    allocations: {
        bo_id: string;
        line_id: string;
        amount: string;
    }[];
};
// Call within the consuming transaction. Draft revisions do not replace the
// approved baseline; the shared project lock protects the decision and its notice.
export async function effectiveApproved(db: DB, target: string): Promise<ApprovedSource | null> {
    await project(db, target, true);
    const row = (await db.query('SELECT is_active,approved_snapshot FROM project_budgets WHERE project_id=$1', [target])).rows[0], a: Approved | null = row?.is_active ? row.approved_snapshot : null;
    return a ? {
        revision: a.revision, currency: a.data.currency, decision_id: a.decision_id, allocations: a.data.allocations.filter(a => a.is_active).map(a => ({ bo_id: a.bo_id, line_id: a.line_id, amount: a.amount }))
    } : null;
}
