import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction } from '../../db';
import { assignmentIdentity, assignmentIdentities } from '../../auth/access';
import { actor, project, readable, assignable, forbidden } from '../core/access';
import { PurchaseError, uuid, reason, version } from '../core/validation';
import { timeSQL, timestamp } from '../../inbox/repository';
import { workspace } from './repository';
const stale = () => new PurchaseError(409, 'purchase ownership changed; refresh before saving');
export async function options(token: string, target: string) {
    return transaction(async (db) => {
        const user = await actor(db, token), p = await project(db, target), w = await workspace(db, p.id);
        if (!readable(user, p, w) || !assignable(user, p))
            throw forbidden();
        return (await assignmentIdentities(db, [])).filter(u => u.allows('projects.purchase_workflow.view') && u.allows('projects.purchase_workflow.read')).map(u => ({ id: u.id, name: u.name }));
    });
}
export async function assign(token: string, target: string, input: Record<string, unknown>) {
    return transaction(async (db) => {
        const user = await actor(db, token), manager = uuid(input.purchase_manager_id, 'purchase_manager_id', 'Select an eligible purchase manager'), expected = version(input.expected_version, 'expected_version'), explanation = reason(input.reason);
        const p = await project(db, target, true), before = await workspace(db, p.id);
        if (!readable(user, p, before) || !assignable(user, p))
            throw forbidden();
        if (before.version !== expected)
            throw stale();
        if (before.purchase_manager_id === manager)
            throw new PurchaseError(422, 'Select a different purchase manager', 'purchase_manager_id');
        const candidate = await assignmentIdentity(db, manager);
        if (!candidate?.active || !candidate.allows('projects.purchase_workflow.view') || !candidate.allows('projects.purchase_workflow.read'))
            throw new PurchaseError(422, 'Select an active user with purchase view and read permissions', 'purchase_manager_id');
        const result = expected === 0 ? await db.query('INSERT INTO purchase_fms_workspaces(project_id,purchase_manager_id,created_by_id,updated_by_id)VALUES($1,$2,$3,$3) RETURNING project_id', [p.id, manager, user.id])
            : await db.query('UPDATE purchase_fms_workspaces SET purchase_manager_id=$2,updated_by_id=$3,updated_at=clock_timestamp(),version=version+1 WHERE project_id=$1 AND version=$4 AND is_active RETURNING project_id', [p.id, manager, user.id, expected]);
        if (!result.rowCount)
            throw stale();
        const saved = await workspace(db, p.id), event = randomUUID();
        const row = (await db.query(`INSERT INTO purchase_fms_owner_history(id,project_id,previous_manager_id,purchase_manager_id,workspace_version,reason,created_by_id)VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING ${timeSQL('created_at')} AS created_at`, [event, p.id, before.purchase_manager_id, manager, saved.version, explanation, user.id])).rows[0];
        await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,scope_id,visibility,occurred_at)VALUES($1,'purchase_fms','purchase_workspace',$2,$3,0,'owner_assigned',$4,'user',$1,'project',$2,'restricted',$5)`, [event, p.id, saved.version, user.id, timestamp(row.created_at)]);
        return saved;
    });
}
