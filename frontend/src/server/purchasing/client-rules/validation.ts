import { object, string, version, reason, uuid, isoDate, trim, invalid } from '../core/validation';
import type { Data } from './types';
export function input(raw: Record<string, unknown>) { const q = object(raw, ['data', 'version', 'is_active', 'reason']), d = object(q.data, ['project_id', 'scope_code', 'mode', 'allowed', 'excluded', 'effective_from', 'effective_until', 'evidence']); if (q.is_active === undefined)
    q.is_active = true; if (typeof q.is_active !== 'boolean')
    invalid('is_active', 'Use true or false'); for (const k of ['data', 'version', 'is_active', 'reason'])
    if (q[k] === null)
        invalid(k, 'This field must not be null'); for (const k of ['scope_code', 'mode', 'allowed', 'excluded', 'effective_from', 'evidence'])
    if (d[k] === null)
        invalid(k, 'This field must not be null'); const data: Data = { project_id: d.project_id == null ? null : uuid(d.project_id), scope_code: trim(string(d.scope_code, 'scope_code', '')), mode: string(d.mode, 'mode', ''), allowed: [], excluded: [], effective_from: string(d.effective_from, 'effective_from', ''), effective_until: d.effective_until == null ? null : string(d.effective_until, 'effective_until', ''), evidence: trim(string(d.evidence, 'evidence', '')) }; if (!data.scope_code || [...data.scope_code].length > 40)
    invalid('scope_code', 'Choose a category, configuration or *'); if (!['unrestricted', 'approved_only'].includes(data.mode))
    invalid('mode', 'Choose unrestricted or approved_only'); if (!data.evidence || [...data.evidence].length > 2000)
    invalid('evidence', 'Provide client policy evidence within 2000 characters'); if (!isoDate(data.effective_from))
    invalid('effective_from', 'Use a valid ISO date'); if (data.effective_until !== null && (!isoDate(data.effective_until) || data.effective_until < data.effective_from))
    invalid('effective_until', 'Rule expiry must not precede its start'); for (const k of ['allowed', 'excluded'] as const) {
    const values = d[k] ?? [];
    if (!Array.isArray(values) || values.length > 200)
        invalid(k, 'Use at most 200 manufacturers in each list');
    const seen = new Set<string>();
    data[k] = values.map(v => { const id = uuid(v); if (seen.has(id))
        invalid(k, 'Duplicate manufacturer'); if (k === 'excluded' && data.allowed.includes(id))
        invalid(k, 'A manufacturer cannot be both allowed and excluded'); seen.add(id); return id; });
} if (data.mode === 'approved_only' && !data.allowed.length)
    invalid('allowed', 'An approved-only rule requires manufacturers'); return { data, version: version(q.version ?? 1, 'version', 1), is_active: q.is_active, reason: reason(q.reason, 'Provide a reason within 2000 characters') }; }
