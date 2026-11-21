import { PurchaseError, invalid, trim, isoDate } from '../core/validation';
import { id } from '../approvals/validation';
import { indiaDate } from '../../personal-tasks/validation';
import type { Member, Source, Package, Assignment, Step, Capabilities, Command } from './types';
export const missing = () => new PurchaseError(404, 'Purchase package or source not found');
export const forbidden = () => new PurchaseError(403, 'Purchase package permission or task scope denied');
export const stale = () => new PurchaseError(409, 'Package changed; retain your draft, reload and review before saving');
export const duplicate = () => new PurchaseError(409, 'This released indent line already has a purchase package');
export function text(field: string, value: string, max: number, required = true) { value = trim(value); if (required && !value || [...value].length > max)
    invalid(field, 'Use a nonempty value within the field limit'); return value; }
export function guard(s: Source) { if (!s.project_available)
    invalid('project', 'Project is inactive, held, closed or purchasing is disabled'); if (!s.eligible || s.blockers.length)
    invalid('source', 'Purchase preparation blocked: ' + s.blockers.join('; ')); }
export function workGuard(actor: string, c: Capabilities, s: Step, action: string) { if (s.number < 3 || s.number > 20)
    invalid('step', 'Steps1–2 are linked source records; use item and indent controls'); if (!['save', 'start', 'block', 'resolve'].includes(action))
    invalid('action', 'Specialist completion is not inferred from preparation'); if (!(action === 'resolve' ? c.resolve : c.execute) || ((action === 'resolve' ? s.blocked_resolver_id : s.doer_id) !== actor && !c.recovery))
    throw forbidden(); if (!s.doer_id || !s.resolver_id || s.doer_id === s.resolver_id || !s.team || s.planned_date === null)
    invalid('assignment', 'Assign team, distinct doer/resolver and planned date first'); if (action === 'resolve' && s.state !== 'blocked')
    invalid('step', 'This step has no open blocker'); if (action !== 'resolve' && s.state === 'blocked')
    invalid('step', 'Resolve the captured blocker before editing preparation'); }
export function assign(raw: Package, rows: Assignment[], members: Member[], actor: string, at: string) { const p = structuredClone(raw); if (p.steps.length !== 20)
    invalid('package', 'Invalid package step catalogue'); if (rows.length < 1 || rows.length > 18)
    invalid('rows', 'Assign1–18 preparation steps'); const seen = new Set<number>(); for (const row of rows) {
    if (row.number < 3 || row.number > 20 || !Number.isInteger(row.number) || seen.has(row.number))
        invalid('rows', 'Use unique preparation steps3–20');
    seen.add(row.number);
    try {
        id(row.doer_id);
        id(row.resolver_id);
    }
    catch {
        invalid('resolver_id', 'Choose distinct active doer and resolver');
    }
    if (row.doer_id === row.resolver_id)
        invalid('resolver_id', 'Choose distinct active doer and resolver');
    const team = text('team', row.team, 120);
    if (!isoDate(row.planned_date))
        invalid('planned_date', 'Choose a planned date');
    if (!members.some(m => m.id === row.doer_id && m.can_execute) || !members.some(m => m.id === row.resolver_id && m.can_resolve))
        invalid('rows', 'Choose active users with live matching workflow permissions');
    const step = p.steps[row.number - 1];
    if (step.state === 'blocked')
        invalid('rows', 'Resolve the captured blocker before reassignment');
    Object.assign(step, { team, doer_id: row.doer_id, resolver_id: row.resolver_id, planned_date: row.planned_date, updated_at: at, updated_by_id: actor });
} return p; }
export function command(raw: Package, number: number, c: Command, actor: string, caps: Capabilities, at: string) { const p = structuredClone(raw); if (p.steps.length !== 20)
    invalid('package', 'Invalid package step catalogue'); if (!Number.isInteger(number) || number < 1 || number > 20)
    invalid('step', 'Unknown purchase step'); const s = p.steps[number - 1]; workGuard(actor, caps, s, c.action); const notes = text('notes', c.notes, 4000, false); if (c.action === 'block') {
    if (!c.resolution_due || !isoDate(c.resolution_due) || c.resolution_due < indiaDate(new Date(at)))
        invalid('resolution_due', 'A resolver deadline today or later is required');
    Object.assign(s, { state: 'blocked', block_reason: c.reason, notes, blocked_at: at, blocked_resolver_id: s.resolver_id, resolution_due: c.resolution_due });
}
else if (c.action === 'resolve')
    Object.assign(s, { state: 'preparing', block_reason: '', blocked_at: null, blocked_resolver_id: null, resolution_due: null });
else
    Object.assign(s, { state: 'preparing', notes }); s.updated_at = at; s.updated_by_id = actor; return p; }
