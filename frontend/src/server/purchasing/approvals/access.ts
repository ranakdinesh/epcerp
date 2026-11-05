import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { project, scope, type Actor, type Project } from '../core/access';
import { projectAccess } from '../core/project-access';
import { workspace } from '../workspace/repository';
import { AppError } from '../../errors';
import type { Capabilities } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.group_comparisons.view', 'accounts.quotation_approvals.view', 'projects.master.read'], true);
export async function actions(db: DB, a: Actor, target: string): Promise<{
    caps: Capabilities;
    project: Project;
}> {
    const p = await project(db, target, true), w = await workspace(db, p.id);
    const check = async (code: string, write: boolean, purchase = false) => { if (!a.allows(code))
        return false; if (purchase && (!a.allows('projects.purchase_workflow.view') || !a.allows('projects.purchase_workflow.read')))
        return false; try {
        const value = purchase ? await scope(db, a, p, w, code, write) : await projectAccess(db, a, p, code, write);
        return !write || value.can_write;
    }
    catch (e) {
        if (e instanceof AppError && [403, 422].includes(e.status))
            return false;
        throw e;
    } };
    const pv = await check('projects.group_comparisons.view', false, true), pr = await check('projects.group_comparisons.read', false, true), pw = await check('projects.group_comparisons.write', true, true), av = await check('accounts.quotation_approvals.view', false), ar = await check('accounts.quotation_approvals.read', false), aw = await check('accounts.quotation_approvals.write', true), vv = await check('accounts.vendor_verification.view', false), vr = await check('accounts.vendor_verification.read', false), br = await check('projects.budgets.read', false), bw = await check('projects.budgets.write', true);
    return { project: p, caps: { read: pv && pr || av && ar, submit: pv && pr && pw, review_read: av && ar, review: av && ar && aw, verify: vv && vr, budget_read: br, budget_write: br && bw } };
}
