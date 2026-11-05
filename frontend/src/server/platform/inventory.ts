import type { DB } from '../db';
import queries from './inventory-sql.json';
export type Work = {
    Source: string;
    Kind: string;
    ID: string;
    OwnerID: string;
    Version: number;
    Title: string;
    State: string;
    Link: string;
    DueDate: string;
    RequiredPermissions: string[];
    ProjectID?: string;
};
export const workView = (w: Work) => ({
    source: w.Source, kind: w.Kind, id: w.ID, owner_id: w.OwnerID, title: w.Title, state: w.State, link: w.Link, due_date: w.DueDate ?? '', version: w.Version, required_permissions: w.RequiredPermissions ?? []
});
export async function inventory(db: DB, owner = '', ref?: Work) {
    const out: Work[] = [];
    if (!ref || ref.Source === 'auth') {
        const reports = (await db.query("SELECT r.user_id AS id,r.manager_id AS owner,u.username AS title,r.version FROM auth.auth_reporting_lines r JOIN auth.users u ON u.id=r.user_id WHERE r.is_active AND u.is_active AND r.manager_id IS NOT NULL AND($1=''OR r.manager_id::text=$1)", [owner])).rows;
        const departments = (await db.query("SELECT id,manager_id AS owner,name AS title,version FROM auth.auth_departments WHERE is_active AND manager_id IS NOT NULL AND($1=''OR manager_id::text=$1)", [owner])).rows;
        for (const [kind, rows] of [['reporting_manager', reports], ['department_manager', departments]] as const)
            for (const r of rows)
                if (!ref || ref.Kind === kind && ref.ID === r.id)
                    out.push({
                        Source: 'auth', Kind: kind, ID: r.id, OwnerID: r.owner, Title: r.title + ' · ' + kind.replace('_', ' '), State: 'active', Link: '/dashboard?screen=organisation', Version: Number(r.version), RequiredPermissions: [], DueDate: ''
                    });
    }
    for (const source of Object.keys(queries) as (keyof typeof queries)[]) {
        if (ref && ref.Source !== source)
            continue;
        const rows = (await db.query(queries[source], [owner, ref?.Kind ?? '', ref?.ID ?? ''])).rows;
        if (rows.length > 10000)
            throw Error('Work inventory exceeds reviewed bounds');
        for (const row of rows) {
            const w = row.payload as Work;
            w.Version = Number(w.Version);
            w.DueDate ??= '';
            if (['purchase_fms', 'project_access', 'project_budgets'].includes(source)) {
                const p = (await db.query('SELECT name,is_active,lifecycle_status FROM projects WHERE id=$1', [w.ProjectID])).rows[0];
                if (!p)
                    throw Error('Work parent context missing');
                if (!p.is_active || p.lifecycle_status === 'closed')
                    continue;
                w.Title = p.name + ' · ' + (w.Kind === 'purchasing_owner' ? 'purchasing ownership' : w.Title);
            }
            out.push(w);
        }
    }
    return out;
}
