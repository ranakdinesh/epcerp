import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import type { Actor } from '../core/access';
import { invalid } from '../core/validation';
import { now } from '../core/time';
import { indiaDate } from '../../personal-tasks/validation';
import { id, comparisons, allocation, total } from './values';
import { input } from './validation';
import { guard, accessGuard, forbidden, missing, stale, selected, apply } from './policy';
import * as context from './context';
import * as repo from './repository';
import type { Proof, Supplier, Member } from './types';
export async function workspaceBound(db: DB, a: Actor, p: string, id: string) {
    const caps = await context.access(db, a, p, id);
    if (!caps.read)
        throw forbidden();
    const source = await context.source(db, p, id);
    if (source.package_id !== id)
        invalid('source', 'Source does not belong to this package');
    const record = await repo.get(db, p, id);
    if (record && !record.is_active)
        throw missing();
    const out = { approved_evidence: [] as Awaited<ReturnType<typeof context.evidence>>, case: record, source, suppliers: [] as Supplier[], approvers: [] as Member[], actions: [] as string[], comparison: record ? comparisons(record, indiaDate(new Date())) : [], history: !caps.scoped || !record ? [] : await repo.history(db, p, id), task_only: !caps.scoped };
    try {
        guard(source);
    }
    catch {
        if (record && ['pending', 'approved'].includes(record.state)) {
            try {
                accessGuard(a, caps, record, 'reopen');
                out.actions.push('reopen');
            }
            catch { }
        }
        return out;
    }
    if (caps.configure || caps.approve || caps.work[4] || caps.work[5])
        out.approved_evidence = await context.evidence(db, p, id);
    const members = await context.approvers(db, p);
    if (caps.configure)
        out.approvers = members;
    if (caps.write && (caps.configure || caps.work[3] || caps.work[4] || caps.work[5]))
        out.suppliers = await context.suppliers(db, p, id);
    for (const action of ['configure', 'rfq', 'dispatch', 'no_bid', 'quote', 'propose', 'decide', 'reopen', 'release']) {
        try {
            accessGuard(a, caps, record, action);
        }
        catch {
            continue;
        }
        if (!record) {
            if (action === 'configure')
                out.actions.push(action);
            continue;
        }
        if (record.award_reference && action !== 'release')
            continue;
        const state = record.state, valid = state === 'draft' && ['configure', 'rfq', 'dispatch', 'no_bid', 'quote', 'propose'].includes(action) || state === 'pending' && ['decide', 'reopen'].includes(action) || state === 'approved' && ['release', 'reopen'].includes(action);
        if (valid && (action !== 'decide' || members.some(m => m.id === a.id)))
            out.actions.push(action);
    }
    return out;
}
export async function workspace(token: string, p: string, record: string) { return transaction(async (db) => { const a = await context.actor(db, token); id(a.id); id(p); id(record); return workspaceBound(db, a, p, record); }); }
export async function execute(token: string, p: string, record: string, raw: Record<string, unknown>) {
    return transaction(async (db) => {
        const a = await context.actor(db, token);
        id(a.id);
        id(p);
        id(record);
        const q = input(raw), caps = await context.access(db, a, p, record);
        if (!caps.read)
            throw forbidden();
        const before = await repo.get(db, p, record);
        if (before?.award_reference && q.action !== 'release')
            invalid('action', 'Award carries a completed Finance decision; revise the source quotation through its owning workflow');
        accessGuard(a, caps, before, q.action);
        if (q.version !== (before?.version ?? 0))
            throw stale();
        const src = await context.source(db, p, record);
        if (src.package_id !== record)
            invalid('source', 'Source does not belong to this package');
        const suppliers = q.action === 'reopen' ? [] : await context.suppliers(db, p, record), members = q.action === 'reopen' ? [] : await context.approvers(db, p);
        let proof: Proof | null = null;
        if (['propose', 'decide', 'release'].includes(q.action)) {
            if (!before)
                throw missing();
            const candidate = structuredClone(before);
            if (q.action === 'propose')
                candidate.selected_offer_id = q.offer_id;
            proof = await context.verify(db, p, record, selected(candidate, indiaDate(new Date())));
        }
        const next = apply(before, a, caps, q, src, suppliers, members, proof, now(), randomUUID(), randomUUID(), p);
        let debit: ReturnType<typeof allocation> | null = null;
        if (next.state !== 'draft') {
            if (!proof)
                invalid('financial_approval', 'Current approval evidence is required for reservation');
            const used = await repo.balances(db, p, next.source_snapshot.demand_id, next.id, proof.reference + ':' + proof.fingerprint);
            debit = allocation(next.source_snapshot.quantity, proof.quantity, proof.total, used.quantity, used.amount, total(selected(next, indiaDate(new Date())), next.source_snapshot.quantity));
        }
        const saved = await repo.save(db, next, a.id);
        await repo.reserve(db, saved, proof, debit);
        await repo.event(db, before, saved, q.action, q.reason, a.id);
        return workspaceBound(db, a, p, record);
    });
}
