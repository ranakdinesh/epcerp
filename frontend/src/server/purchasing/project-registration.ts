import 'server-only';
import { randomUUID } from 'node:crypto';
import type { DB } from '../db';
import { assignmentIdentity } from '../auth/access';
import { ProjectError } from '../projects/validation';
import { procurementManagerProjectNotice } from '../inbox/purchasing-notices';
// Published initial workspace writer; must share the Project creation transaction.
export async function initializeProjectPurchase(db: DB, project: string, user: string | null | undefined, actor: string) {
    const assignedUser = user && typeof user === 'string' && user.trim() !== '' ? user.trim() : null;
    if (assignedUser) {
        const u = await assignmentIdentity(db, assignedUser);
        if (!u?.active || !['projects.purchase_workflow.view', 'projects.purchase_workflow.read'].every(p => u.allows(p)))
            throw new ProjectError(422, 'Select an active Procurement Executive with purchase view/read permissions', 'procurement_executive_id');
    }
    await db.query('INSERT INTO purchase_fms_workspaces(project_id,purchase_manager_id,created_by_id,updated_by_id)VALUES($1,$2,$3,$3)', [project, assignedUser, actor]);
    const event = randomUUID();
    const reasonText = assignedUser ? 'Initial Procurement Executive assignment at project creation' : 'Initial unassigned Purchase FMS workspace awaiting assignment';
    const row = (await db.query("INSERT INTO purchase_fms_owner_history(id,project_id,previous_manager_id,purchase_manager_id,workspace_version,reason,created_by_id)VALUES($1,$2,NULL,$3,1,$4,$5) RETURNING created_at::text AS at", [event, project, assignedUser, reasonText, actor])).rows[0];
    await db.query(`INSERT INTO business_events(id,source_module,entity_type,entity_id,entity_version,delivery_count,action,actor_id,actor_kind,correlation_id,scope_kind,scope_id,visibility,occurred_at)VALUES($1,'purchase_fms','purchase_workspace',$2,1,0,'owner_assigned',$3,'user',$1,'project',$2,'restricted',$4)`, [event, project, actor, row.at]);

    const p = (await db.query('SELECT project_number, name FROM projects WHERE id=$1', [project])).rows[0];
    if (p) {
        await procurementManagerProjectNotice(db, event, project, p.project_number, p.name);
    }
}
