import type { DB } from '../../db';
import { timeSQL, timestamp } from '../../inbox/repository';
import type { Workspace } from '../core/access';
export async function workspace(db: DB, project: string): Promise<Workspace> {
    const row = (await db.query(`SELECT project_id,purchase_manager_id,version,is_active,${timeSQL('updated_at')} AS updated_at,updated_by_id FROM purchase_fms_workspaces WHERE project_id=$1`, [project])).rows[0];
    return row ? { ...row, updated_at: timestamp(row.updated_at)! } as Workspace : { project_id: project, purchase_manager_id: null, version: 0, is_active: false, updated_at: '0001-01-01T00:00:00Z', updated_by_id: null };
}
export async function history(db: DB, project: string) {
    return (await db.query(`SELECT id,previous_manager_id,purchase_manager_id,created_by_id,reason,workspace_version AS version,${timeSQL('created_at')} AS created_at FROM purchase_fms_owner_history WHERE project_id=$1 ORDER BY workspace_version DESC`, [project])).rows.map(row => ({ ...row, created_at: timestamp(row.created_at) }));
}
