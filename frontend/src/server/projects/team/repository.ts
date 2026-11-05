import type { DB } from '../../db';
import { timeSQL, timestamp } from '../../inbox/repository';
import { missing, invalid } from './validation';
export type Profile = {
    id: string; project_id: string; user_id: string; user_name: string; bundle_id: string; bundle_name: string;
    responsibility: string; permission_codes: string[]; is_active: boolean; version: number;
    created_at: string; updated_at: string; created_by_id: string | null; updated_by_id: string | null;
};
export const exact = `${timeSQL('created_at')} AS created_at,${timeSQL('updated_at')} AS updated_at`;
export function profile(row: Record<string, unknown>): Profile {
    return { id: String(row.id), project_id: String(row.project_id), user_id: String(row.user_id), user_name: '', bundle_id: String(row.bundle_id), bundle_name: '', responsibility: String(row.responsibility), permission_codes: row.permission_codes as string[], is_active: row.is_active === true, version: Number(row.version), created_at: timestamp(row.created_at as string)!, updated_at: timestamp(row.updated_at as string)!, created_by_id: row.created_by_id as string | null, updated_by_id: row.updated_by_id as string | null };
}
export async function list(db: DB, project: string) {
    const rows = (await db.query(`SELECT *,${exact} FROM project_responsibility_profiles WHERE project_id=$1 ORDER BY bundle_id,id LIMIT 1001`, [project])).rows;
    if (rows.length > 1000) invalid('profiles', 'Project responsibility limit reached');
    return rows.map(profile);
}
export async function get(db: DB, project: string, id: string) {
    const row = (await db.query(`SELECT *,${exact} FROM project_responsibility_profiles WHERE project_id=$1 AND id=$2`, [project, id])).rows[0];
    if (!row) throw missing();
    return profile(row);
}
