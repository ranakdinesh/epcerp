import 'server-only';
import { tenderAwardPending } from '../core/tender-award';
import { transaction, type DB } from '../../db';
import { actor, project, readable, portfolio, assignable, forbidden, type Actor } from '../core/access';
import { invalid, trim, uuid } from '../core/validation';
import { workspace, history } from './repository';
import { source } from '../progress/source';
async function row(db: DB, a: Actor, id: string) { const p = await project(db, id, true), w = await workspace(db, id); if (!readable(a, p, w))
    throw forbidden(); const progress = (await source(db, id)).register; return { ...(await tenderAwardPending(db, id) ? { tender_award_pending: true } : {}), id: p.id, project_number: p.project_number, name: p.name, project_manager_id: p.project_manager_id, lifecycle_status: p.lifecycle_status, is_active: p.is_active, purchase_manager_id: w.purchase_manager_id, ownership_version: w.version, can_assign: assignable(a, p), progress_percent: progress.percent, progress_status: 'purchase_stages_credit', recorded_packages: null, completed_steps: progress.completed, next_step: progress.next, pending_approval_groups: progress.pending, returned_groups: progress.returned, approved_groups: progress.approved }; }
export async function context(token: string, p: string) { return transaction(async (db) => { const a = await actor(db, token); p = uuid(p); return { project: await row(db, a, p), history: await history(db, p) }; }); }
export function filter(q: URLSearchParams) { for (const key of q.keys())
    if (!['page', 'page_size', 'search', 'sort', 'direction', 'enabled_only'].includes(key) || q.getAll(key).length !== 1)
        invalid('query', 'Unsupported query parameter'); const integer = (key: string, fallback: number) => { const s = q.get(key); if (s !== null && !/^[+-]?\d+$/.test(s))
    invalid('query', 'Use integer pagination'); return s === null ? fallback : Number(s); }, page = integer('page', 1), size = integer('page_size', 20), search = trim(q.get('search') ?? '').toLowerCase(), sort = q.get('sort') || 'number', direction = q.get('direction') || 'asc'; if (!Number.isInteger(page) || page < 1 || page > 100000 || !Number.isInteger(size) || size < 1 || size > 100 || [...search].length > 255)
    invalid('query', 'Invalid pagination or search'); if (!['number', 'name', 'state'].includes(sort) || !['asc', 'desc'].includes(direction))
    invalid('query', 'Unsupported sort'); const enabledOnly = q.get('enabled_only'); if (enabledOnly !== null && enabledOnly !== 'true')
    invalid('query', 'enabled_only must be true'); return { page, size, search, sort, direction, enabledOnly: enabledOnly === 'true' }; }
export async function dashboard(token: string, q = new URLSearchParams()) {
    const countScope = q.getAll('include_scope_total');
    if (countScope.length && (countScope.length !== 1 || countScope[0] !== 'true'))
        invalid('query', 'include_scope_total must be true');
    const filters = new URLSearchParams(q);
    filters.delete('include_scope_total');
    const f = filter(filters);
    return transaction(async db => {
        const a = await actor(db, token);
        const all = portfolio(a);
        const search = f.search ? '%' + f.search.replace(/[\\%_]/g, c => '\\' + c) + '%' : '';
        const scope = `p.is_active AND ($1::boolean OR p.project_manager_id=$2 OR EXISTS(
            SELECT 1 FROM purchase_fms_workspaces w
            WHERE w.project_id=p.id AND w.purchase_manager_id=$2 AND w.is_active
        ))${f.enabledOnly ? ' AND p.purchase_enabled' : ''}`;
        const matches = "($3::text='' OR lower(p.name) LIKE $3 OR lower(p.project_number) LIKE $3)";
        const args = [all, a.id, search];
        const counts = (await db.query(countScope.length
            ? `SELECT count(*) FILTER (WHERE ${matches}) AS total, count(*) AS scope_total FROM projects p WHERE ${scope}`
            : `SELECT count(*) AS total FROM projects p WHERE ${scope} AND ${matches}`, args)).rows[0];
        const order = { number: 'p.project_number', name: 'lower(p.name)', state: '(NOT p.is_active)' }[f.sort as 'number' | 'name' | 'state'];
        const ids = (await db.query(`SELECT p.id FROM projects p WHERE ${scope} AND ${matches}
            ORDER BY ${order} ${f.direction},p.id LIMIT $4 OFFSET $5`,
            [...args, f.size, (f.page - 1) * f.size])).rows.map(r => r.id as string);
        const byID = new Map<string, Awaited<ReturnType<typeof row>>>();
        for (const id of [...ids].sort())
            byID.set(id, await row(db, a, id));
        return {
            items: ids.map(id => byID.get(id)),
            total: Number(counts.total),
            page: f.page,
            page_size: f.size,
            portfolio: all,
            ...(countScope.length ? { scope_total: Number(counts.scope_total) } : {}),
        };
    });
}
