import type { DB } from '../../db';
import type { Hold } from '../indents/types';
export type Data = {
    scope: string;
    category_code: string;
    demand_ids: string[];
    reason: string;
    evidence: string;
    doer_id: string;
    resolver_id: string;
    started_at: string;
    expected_release_at: string | null;
    released_at: string | null;
    release_evidence: string | null;
    recorded_at: string;
};
export async function holdEvidence(db: DB, project: string, demand: string, category: string, now = Date.now()) {
    const records = (await db.query('SELECT id,data,is_active FROM purchase_holds WHERE project_id=$1 ORDER BY created_at,id', [project])).rows;
    const holds: Hold[] = [], spans: {
        a: number;
        b: number;
    }[] = [];
    for (const r of records) {
        const d = r.data as Data;
        if (!(d.scope === 'project' || d.scope === 'category' && d.category_code === category || d.scope === 'items' && d.demand_ids.includes(demand)))
            continue;
        const start = Date.parse(d.started_at), end = Math.min(now, d.released_at ? Date.parse(d.released_at) : now);
        if (r.is_active && start <= now)
            holds.push({ id: r.id, reason: d.reason, doer_id: d.doer_id, resolver_id: d.resolver_id, started_at: d.started_at });
        if (end > start)
            spans.push({ a: start, b: end });
    }
    spans.sort((a, b) => a.a - b.a);
    const merged: typeof spans = [];
    for (const span of spans) {
        const last = merged.at(-1);
        if (last && span.a <= last.b)
            last.b = Math.max(last.b, span.b);
        else
            merged.push({ ...span });
    }
    return { holds, hold_minutes: Math.round(merged.reduce((total, s) => total + (s.b - s.a) / 60000, 0) * 100) / 100, finish_impact: 'Unassessed' };
}
