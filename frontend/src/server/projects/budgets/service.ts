import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import { projectAccess } from '../../purchasing/core/project-access';
import { now } from '../../purchasing/core/time';
import { actor, context, sources, capabilities, type Actor } from './context';
import { command } from './input';
import { validate, report, priceItems, attribute, emptyAudit } from './policy';
import { money } from './money';
import { id, invalid, stale, forbidden, missing } from './validation';
import type { Budget, Workspace } from './types';
import * as repo from './repository';
import * as finance from './finance';
async function view(db: DB, a: Actor, target: string): Promise<Workspace> {
    const { p, project } = await context(db, target), { owners, items } = await sources(db, p), record = await repo.get(db, target), caps = await capabilities(db, a, p);
    return {
        record, project, owners, items, can_write: caps.write && project.available && a.allows('projects.budgets.write'), can_approve: caps.approve && project.available && a.allows('projects.budgets.approve'), report: record ? report(record.data) : null, approved_report: record?.approved ? report(record.approved.data) : null, history: await repo.history(db, target)
    };
}
export async function workspace(token: string, target: string) {
    return transaction(async (db) => {
        const a = await actor(db, token), p = await context(db, id(target));
        await projectAccess(db, a, p.p, 'projects.budgets.read', false);
        return view(db, a, p.p.id);
    });
}
export async function execute(token: string, target: string, raw: unknown) {
    return transaction(async (db) => {
        const a = await actor(db, token), projectID = id(target), input = command(raw), permission = input.action === 'approve' ? 'projects.budgets.approve' : 'projects.budgets.write';
        if (!a.allows(permission))
            throw forbidden();
        const { p, project } = await context(db, projectID, true);
        await projectAccess(db, a, p, permission, true);
        if (!project.available)
            invalid('project', 'Project is inactive, held or closed');
        const scope = await sources(db, p), existing = await repo.get(db, projectID), before = structuredClone(existing), previous = existing ? structuredClone(existing.data) : null;
        const b: Budget = existing ?? {
            ...emptyAudit(), project_id: projectID, version: 0, revision: 1, state: 'draft', is_active: true, approved: null, data: {
                total: '', currency: '', owner_id: '', notes: '', heads: [], allocations: []
            }
        };
        if (!b.is_active)
            throw missing();
        if (b.version !== input.version)
            throw stale();
        if (input.action === 'revise') {
            if (b.state !== 'approved' || !b.approved)
                invalid('state', 'Only an approved budget can start a revision');
            b.revision++;
            b.state = 'draft';
        }
        else {
            if (b.state !== 'draft')
                invalid('state', 'Start a revision before changing an approved budget');
            if (project.contract_value === null)
                invalid('contract_value', 'Ask an administrator to enter the total project value first');
            let data = structuredClone(input.action === 'save' ? input.data! : b.data);
            if (input.action === 'price_items')
                data = priceItems(previous, input.prices, scope.items, a.id, project.contract_value, project.currency, randomUUID);
            let total: bigint;
            try {
                total = money(data.total);
            }
            catch {
                invalid('data', 'Save a valid budget draft first');
            }
            if (total > money(project.contract_value))
                invalid('total', 'Project budget cannot exceed the total project value');
            data = validate(data, project.currency, scope.owners, scope.items, previous);
            if (input.action === 'approve') {
                if (total <= BigInt(0) || !data.heads.some(h => h.is_active))
                    invalid('data', 'Approve a positive project budget with at least one head');
                const decision = await finance.approve(db, a, { ...b, data }, input.reason);
                b.state = 'approved';
                b.approved = {
                    revision: b.revision, data: structuredClone(data), approved_by_id: a.id, approved_at: now(), decision_id: decision
                };
            }
            else
                data = attribute(data, previous, a.id, now());
            b.data = data;
        }
        b.version++;
        const saved = await repo.save(db, b, a.id);
        await repo.event(db, before, saved, input.action, input.reason, a.id);
        return view(db, a, projectID);
    });
}
