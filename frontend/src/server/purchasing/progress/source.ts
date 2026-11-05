import type { DB } from '../../db';
import { invalid } from '../core/validation';
import { list as bos } from '../bo/repository';
import * as workflowRepo from '../workflow/repository';
import { source as workSource } from '../workflow/context';
import { get as groups } from '../groups/repository';
import { grouped } from './grouped';
import { packageFacts } from './package-facts';
import { laterCompletion, registerCompletion, type RegisterState } from './policy';
import { source as approvalSource } from '../approvals/source';
import { get as approvalRecord } from '../approvals/repository';
import { evaluate } from '../approvals/policy';
import { list as documents } from '../indent-documents/repository';
import type { Source, Package, BO } from './types';
export async function source(db: DB, p: string) { const project = (await db.query('SELECT is_active,purchase_enabled,lifecycle_status,planned_start_date::text AS start_date,planned_completion_date::text AS end_date FROM projects WHERE id=$1 FOR UPDATE', [p])).rows[0]; if (!project)
    invalid('project', 'Project not found'); const out: Source = { project_id: p, project_start: project.start_date ?? '', project_end: project.end_date ?? '', bos: [], blockers: [] }; if (!project.is_active || !project.purchase_enabled || ['on_hold', 'closed'].includes(project.lifecycle_status))
    out.blockers.push('Project is inactive, held, closed or purchasing is disabled'); const states: RegisterState[] = []; for (const b of await bos(db, p)) {
    if (!b.is_active)
        continue;
    const row: BO = { id: b.id, name: b.name, state: b.state, lines: b.lines.map(l => ({ id: l.id, demand_id: l.demand_id ?? '', quantity: l.quantity ?? '' })), grouped: await grouped(db, p, b), packages: [] }, demands = row.lines.map(l => l.demand_id).filter(Boolean);
    if (demands.length > 200)
        invalid('demands', 'Use bounded demand/page scope');
    const ids = (await db.query('SELECT id FROM purchase_work_packages WHERE project_id=$1 AND demand_id=ANY($2::uuid[]) AND is_active ORDER BY id LIMIT 1001', [p, demands])).rows;
    if (ids.length > 1000)
        invalid('packages', 'Project reporting package limit exceeded');
    const laterPackages: Package[] = [];
    for (const { id } of ids) {
        const r = (await workflowRepo.get(db, p, id))!, current = await workSource(db, p, r.indent_id, r.demand_id), facts = await packageFacts(db, p, id, true), blockers = !current.eligible || current.demand_id !== r.demand_id || current.quantity !== r.source_snapshot.quantity || current.description !== r.source_snapshot.description || current.uom !== r.source_snapshot.uom ? ['Work package differs from current demand'] : [], pkg: Package = { id: r.id, demand_id: r.demand_id, quantity: r.source_snapshot.quantity, evidence: facts.filter(f => f.number <= 20), coordination: r.steps.filter(s => s.number >= 2).map(s => ({ number: s.number, state: s.state, planned_date: s.planned_date ?? '', block_reason: s.block_reason, doer_id: s.doer_id ?? '', resolver_id: s.resolver_id ?? '', resolution_due: s.resolution_due ?? '' })), source_blockers: blockers };
        row.packages.push(pkg);
        laterPackages.push({ ...pkg, evidence: facts });
    }
    out.bos.push(row);
    const a = await approvalSource(db, p, b.id), r = await approvalRecord(db, p, b.id), e = evaluate(r?.is_active ? r.attempts : [], a), docs = await documents(db, p, b.id);
    let quotesComplete = docs.length > 0;
    for (const d of docs) {
        const gr = await groups(db, p, b.id), current = d.is_active && gr?.is_active && gr.groups.some(g => g.id === d.group_id && g.scope_revision === d.data.scope_revision && g.vendor_ids.includes(d.vendor_id));
        if (current)
            quotesComplete = quotesComplete && a.quotes.some(q => q.document_id === d.id && q.eligible);
    }
    const latest = a.groups.map(g => e.latest.get(g.id)?.effective_state);
    states.push({ state: b.state, has_items: b.lines.length > 0, indent_complete: row.grouped.some(f => f.number === 2 && f.state === 'completed'), quotes_complete: quotesComplete, approval_complete: e.ready, pending: latest.filter(s => s === 'pending_accounts').length, returned: latest.filter(s => s === 'rejected' || s === 'stale').length, approved: latest.filter(s => s === 'approved').length, later_completed: laterCompletion(row.lines, laterPackages) });
} return { source: out, register: registerCompletion(states) }; }
