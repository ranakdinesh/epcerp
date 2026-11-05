import 'server-only';
import { transaction, type DB } from '../../db';
import { actor, project, scope, type Actor } from '../core/access';
import { workspace } from '../workspace/repository';
import { PurchaseError, invalid, uuid, reason, version } from '../core/validation';
import { quantity } from '../core/decimal';
import { catalogue, permitted } from '../core/catalogue';
import { demand } from '../materials/repository';
import { publish } from '../materials/writer';
import type { Record as BORecord, Line, Snapshot } from './types';
import { input, validateLine } from './validation';
import * as repo from './repository';
const missing = () => new PurchaseError(404, 'BO list not found');
const denied = () => new PurchaseError(403, 'BO list permission or project scope denied');
async function access(db: DB, token: string, target: string, permissions: string[], write: boolean) {
    let a: Actor;
    try {
        a = await actor(db, token, permissions);
    }
    catch (e) {
        if (e instanceof PurchaseError && e.status === 403)
            throw denied();
        throw e;
    }
    const p = await project(db, target, true), w = await workspace(db, p.id);
    try {
        await scope(db, a, p, w, permissions.at(-1) ?? 'projects.purchase_bo.read', write);
    }
    catch (e) {
        if (e instanceof PurchaseError && e.status === 403)
            throw denied();
        throw e;
    }
    return { a, p, w };
}
function editable(r: BORecord, a: Actor, expected: number) { if (!r.is_active)
    invalid('', 'BO list is inactive'); if (!a.superuser && r.preparer_id !== a.id)
    throw denied(); if (r.version !== expected)
    throw repo.stale(); if (r.state !== 'draft')
    invalid('', 'Completed BO lists are immutable until explicitly reopened'); }
async function snapshot(db: DB, project: string, id: string): Promise<Snapshot> { const s = await permitted(db, project, await catalogue(db, id)); return { id: s.id, sku: s.sku, description: s.description, engineering_key: s.engineering_key, uom: s.uom, manufacturer: s.manufacturer }; }
const emptySnapshot = (): Snapshot => ({ id: '', sku: '', description: '', engineering_key: '', uom: '', manufacturer: '' });
export async function list(token: string, target: string) {
    return transaction(async (db) => {
        const { a, p, w } = await access(db, token, target, ['projects.purchase_bo.read'], false);
        const writable = async (code: string) => { if (!a.allows(code))
            return false; try {
            await scope(db, a, p, w, code, true);
            return true;
        }
        catch (e) {
            if (e instanceof PurchaseError && [403, 422].includes(e.status))
                return false;
            throw e;
        } };
        return { lists: await repo.list(db, p.id), can_write: await writable('projects.purchase_bo.write'), can_complete: a.allows('projects.items.write') && await writable('projects.purchase_bo.complete') };
    });
}
export async function history(token: string, target: string, id: string) { return transaction(async (db) => { const { p } = await access(db, token, target, ['projects.purchase_bo.read'], false); id = uuid(id); if (!await repo.get(db, p.id, id))
    throw missing(); return repo.history(db, p.id, id); }); }
export async function save(token: string, target: string, id: string, raw: Record<string, unknown>) {
    return transaction(async (db) => {
        const { a, p } = await access(db, token, target, ['projects.purchase_bo.read', 'projects.purchase_bo.write'], true);
        id = uuid(id);
        const b = input(raw), existing = await repo.get(db, p.id, id);
        if (!existing && b.version !== 0)
            throw repo.stale();
        const record: BORecord = existing ?? { id, project_id: p.id, preparer_id: a.id, name: '', state: 'draft', is_active: true, version: 0, lines: [], created_at: '', updated_at: '', completed_at: null, completed_by_id: null, created_by_id: null, updated_by_id: null };
        editable(record, a, b.version);
        const before = existing ? structuredClone(existing) : null;
        if (b.name !== null)
            record.name = b.name;
        const used = new Set<string>();
        for (const other of await repo.list(db, p.id))
            if (other.id !== id && other.is_active)
                for (const l of other.lines)
                    if (l.item_id)
                        used.add(l.item_snapshot.engineering_key);
        const prior = new Map(record.lines.map(l => [l.id, l])), incoming = new Set(b.lines.map(l => l.id));
        for (const line of record.lines)
            if (line.demand_id && !incoming.has(line.id))
                invalid('lines', 'Previously completed items cannot be removed');
        const lines: Line[] = [], seen = new Set<string>();
        for (const row of b.lines) {
            const line: Line = { ...row, item_snapshot: emptySnapshot(), demand_id: null }, old = prior.get(row.id);
            if (old?.demand_id) {
                if (!line.item_id || line.item_id !== old.item_id)
                    invalid('item_id', "A previously completed item's SKU cannot be replaced");
                line.demand_id = old.demand_id;
            }
            if (line.item_id) {
                line.item_snapshot = await snapshot(db, p.id, line.item_id);
                const key = line.item_snapshot.engineering_key;
                if (seen.has(key) || used.has(key))
                    invalid('item_id', "This item already exists in the project's BO lists; edit the existing requirement");
                seen.add(key);
            }
            if (line.demand_id && old?.item_snapshot.demand_version !== undefined)
                line.item_snapshot.demand_version = old.item_snapshot.demand_version;
            validateLine(line, false);
            lines.push(line);
        }
        record.lines = lines;
        record.version++;
        const saved = await repo.save(db, record, a.id);
        await repo.event(db, before, saved, 'save_draft', b.reason, a.id);
        return saved;
    });
}
async function changeRecord(db: DB, a: Actor, target: string, id: string, action: string, expected: number, explanation: string) {
    const p = await project(db, target, true), w = await workspace(db, p.id);
    await scope(db, a, p, w, 'projects.purchase_bo.complete', true);
    await scope(db, a, p, w, 'projects.items.write', true);
    const record = await repo.get(db, p.id, id);
    if (!record)
        throw missing();
    if (action === 'complete' && record.state === 'completed' && record.completed_by_id === a.id && expected === record.version - 1)
        return record;
    const before = structuredClone(record);
    if (action === 'reopen') {
        if (record.state !== 'completed')
            invalid('', 'Only completed BO lists can be reopened');
        record.state = 'draft';
    }
    editable(record, a, expected);
    if (action === 'reopen') {
        for (const line of record.lines) {
            if (!line.demand_id || line.quantity === null || line.required_date === null)
                throw repo.stale();
            const d = await demand(db, p.id, line.demand_id);
            if (!d || !d.is_active || d.item_id !== line.item_id || d.required_date !== line.required_date || quantity(d.quantity) !== quantity(line.quantity))
                throw repo.stale();
            line.item_snapshot.demand_version = d.version;
        }
        record.completed_at = null;
        record.completed_by_id = null;
    }
    else if (action === 'complete') {
        if (!record.lines.length)
            invalid('lines', 'Add at least one item before completing the BO list');
        const keys = new Set<string>();
        for (const line of record.lines) {
            validateLine(line, true);
            const s = await snapshot(db, p.id, line.item_id!);
            if (line.item_snapshot.demand_version !== undefined)
                s.demand_version = line.item_snapshot.demand_version;
            line.item_snapshot = s;
            validateLine(line, true);
            if (keys.has(s.engineering_key))
                invalid('item_id', 'Duplicate engineering requirement; combine quantities or use location allocations');
            keys.add(s.engineering_key);
        }
        for (const line of record.lines) {
            const before = line.demand_id ? await demand(db, p.id, line.demand_id) : null;
            if (line.demand_id && (!before || line.item_snapshot.demand_version === undefined || before.version !== line.item_snapshot.demand_version))
                throw repo.stale();
            const saved = await publish(db, p.id, line.demand_id ?? undefined, { item_id: line.item_id!, quantity: line.quantity!, required_date: line.required_date!, is_active: true, version: before?.version ?? 1, allocations: before?.allocations ?? [], alternatives: before?.alternatives ?? [], reason: explanation }, a.id);
            line.demand_id = saved.id;
        }
        record.state = 'completed';
        record.completed_at = new Date().toISOString();
        record.completed_by_id = a.id;
    }
    else
        invalid('action', 'Choose complete or reopen');
    record.version++;
    const saved = await repo.save(db, record, a.id);
    await repo.event(db, before, saved, action + '_bo_list', explanation, a.id);
    return saved;
}
export async function change(token: string, target: string, id: string, action: string, raw: Record<string, unknown>) { return transaction(async (db) => { const permissions = ['projects.purchase_bo.read', 'projects.purchase_bo.complete', 'projects.items.write']; if (action === 'reopen')
    permissions.push('projects.purchase_bo.write'); const { a, p } = await access(db, token, target, permissions, true); return changeRecord(db, a, p.id, uuid(id), action, version(raw.version, 'version', 1), reason(raw.reason, 'Provide a reason within 2000 characters')); }); }
export async function register(token: string, target: string, action: string, raw: Record<string, unknown>) {
    return transaction(async (db) => {
        const { a, p } = await access(db, token, target, ['projects.purchase_bo.read', 'projects.purchase_bo.write', 'projects.purchase_bo.complete', 'projects.items.write'], true), explanation = reason(raw.reason, 'Provide a reason within 2000 characters');
        if (action !== 'complete' && action !== 'reopen')
            invalid('action', 'Choose complete or reopen');
        if (!Array.isArray(raw.records) || !raw.records.length || raw.records.length > 1000)
            invalid('records', 'Provide the current saved register');
        const expected = new Map<string, number>();
        for (const row of raw.records) {
            if (!row || typeof row !== 'object')
                invalid('records', 'Provide unique saved records and versions');
            const id = uuid(row.id);
            if (expected.has(id))
                invalid('records', 'Provide unique saved records and versions');
            expected.set(id, version(row.version, 'records', 1));
        }
        const records = await repo.list(db, p.id);
        if (records.length !== expected.size || records.some(r => r.version !== expected.get(r.id)))
            throw repo.stale();
        const targets = records.filter(r => r.state === (action === 'complete' ? 'draft' : 'completed'));
        if (!targets.length)
            invalid('records', 'The register is already in this state');
        for (const r of targets)
            editable({ ...r, state: 'draft' }, a, r.version);
        for (const r of targets)
            await changeRecord(db, a, p.id, r.id, action, r.version, explanation);
        return { lists: await repo.list(db, p.id), can_write: true, can_complete: true };
    });
}
export async function printModel(token: string, target: string) { return transaction(async (db) => { const { p } = await access(db, token, target, ['projects.purchase_bo.read'], false), records = await repo.list(db, p.id), comments: string[] = []; for (const r of records) {
    const e = (await repo.history(db, p.id, r.id)).find(e => ['complete_bo_list', 'reopen_bo_list'].includes(e.action));
    if (e)
        comments.push(e.created_at.slice(0, 16).replace('T', ' ') + ' UTC - ' + e.action + ': ' + e.reason);
} return { project: p, records, comments }; }); }
