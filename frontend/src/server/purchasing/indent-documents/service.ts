import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction, type DB } from '../../db';
import { actor, project, scope, open, type Actor, type Project } from '../core/access';
import { requireAnyPermission, assignmentIdentities, assignmentIdentity } from '../../auth/access';
import { workspace as purchaseWorkspace } from '../workspace/repository';
import { get as getBO } from '../bo/repository';
import * as groups from '../groups/repository';
import { executeBound as groupCommand } from '../groups/service';
import { input as groupInput } from '../groups/validation';
import { purchaseVendors } from '../../master/vendors/purchase-reader';
import { PurchaseError, invalid, uuid } from '../core/validation';
import { indiaDate } from '../../personal-tasks/validation';
import { indiaInstant } from '../core/time';
import { input, terms } from './validation';
import type { Document, Data, Command } from './types';
import * as repo from './repository';
const missing = () => new PurchaseError(404, 'Vendor indent not found in this BO');
async function revisions(db: DB, project: string, bo: string) { const b = await getBO(db, project, bo); if (!b?.is_active)
    throw missing(); const r = await groups.get(db, project, bo); return new Map(r?.groups.map(g => [g.id, g.scope_revision]) ?? []); }
async function recipients(db: DB, p: Project) { const users = await assignmentIdentities(db, []); return { manager_assigned: users.some(u => u.id === p.project_manager_id), ids: users.filter(u => u.id === p.project_manager_id || u.superuser).map(u => u.id) }; }
async function workspaceBound(db: DB, a: Actor, p: Project, bo: string, write: boolean) {
    const rev = await revisions(db, p.id, bo), documents = await repo.list(db, p.id, bo), rec = await recipients(db, p);
    for (const d of documents)
        d.data.is_current = d.data.scope_revision === rev.get(d.group_id);
    return { documents, history: await repo.history(db, p.id, bo), manager_assigned: rec.manager_assigned, can_generate: write && a.allows('projects.purchase_indent_documents.generate'), can_print: write && a.allows('projects.purchase_indent_documents.print'), can_dispatch: write && a.allows('projects.purchase_indent_documents.dispatch') };
}
async function access(db: DB, token: string, target: string, permission: string, write: boolean) {
    const a = await actor(db, token, ['projects.purchase_bo.read', 'projects.purchase_indent_groups.read', 'projects.purchase_indent_documents.read', permission]), p = await project(db, target, true), w = await purchaseWorkspace(db, p.id), access = await scope(db, a, p, w, permission, write);
    const client = (await db.query('SELECT is_active FROM clients WHERE id=$1', [p.client_id])).rows[0];
    return { a, p, write: access.can_write && open(p) && p.purchase_enabled && !!client?.is_active };
}
export async function workspace(token: string, target: string, bo: string) { return transaction(async (db) => { const { a, p, write } = await access(db, token, target, 'projects.purchase_indent_documents.read', false); if (!a.allows('projects.purchase_indent_register.view') && !a.allows('projects.purchase_indent_documents.view'))
    throw new PurchaseError(403, 'Vendor indent permission or project scope denied'); return workspaceBound(db, a, p, uuid(bo), write); }); }
async function generate(db: DB, a: Actor, p: Project, bo: string, b: Command): Promise<Data> {
    let record = await groups.get(db, p.id, bo);
    if (!record)
        invalid('group_id', 'Save item groups and vendors first');
    if (record.version !== b.group_version)
        throw repo.stale();
    if (record.state !== 'created') {
        const result = await groupCommand(db, a, p.id, bo, groupInput({ action: 'create', version: record.version, bo_version: b.bo_version, reason: 'Freeze saved groups for vendor indent generation', groups: record.groups.map(g => ({ id: g.id, name: g.name, line_ids: g.line_ids, vendor_ids: g.vendor_ids })) }));
        record = result.record!;
    }
    const vendors = await purchaseVendors(db), g = record.groups.find(g => g.id === b.group_id);
    if (!g)
        invalid('group_id', 'Select a saved item group');
    const selected = vendors.find(v => v.id === b.vendor_id);
    if (g.vendor_ids.filter(id => vendors.some(v => v.id === id)).length < 3)
        invalid('vendor_id', 'Select at least three active vendors before generation');
    if (!selected || !g.vendor_ids.includes(selected.id))
        invalid('vendor_id', 'Choose an active vendor selected for this group');
    const user = await assignmentIdentity(db, a.id);
    if (!user?.active)
        throw new PurchaseError(401, 'Invalid or expired credentials');
    return { terms: terms(selected), vendor: selected, lines: record.source_lines.filter(l => g.line_ids.includes(l.id)), project_name: p.name, generated_by_name: user.name, group_name: g.name, scope_revision: g.scope_revision, number: '', generated_at: indiaInstant(), generated_by_id: a.id, print_requests: [], sent_date: null, channel: '', dispatch_reference: '', sent_recorded_at: null, sent_by_id: null, is_current: false };
}
export async function execute(token: string, target: string, bo: string, raw: Record<string, unknown>) {
    const b = input(raw), permission = 'projects.purchase_indent_documents.' + (b.action === 'send' ? 'dispatch' : b.action);
    return transaction(async (db) => {
        const { a, p, write } = await access(db, token, target, permission, true);
        bo = uuid(bo);
        const rev = await revisions(db, p.id, bo), docs = await repo.list(db, p.id, bo);
        let d: Document, before: Document | null = null, reason = 'Generate frozen vendor indent';
        if (b.action === 'generate') {
            const revision = rev.get(b.group_id!);
            if (revision === undefined)
                invalid('group_id', 'Save item groups and vendors first');
            if (docs.some(old => old.group_id === b.group_id && old.vendor_id === b.vendor_id && old.data.scope_revision === revision))
                return workspaceBound(db, a, p, bo, write);
            const data = await generate(db, a, p, bo, b), id = randomUUID();
            data.number = 'IND-' + id.replaceAll('-', '').toUpperCase();
            d = { id, project_id: p.id, bo_id: bo, group_id: b.group_id!, vendor_id: b.vendor_id!, version: 1, data, is_active: true, created_at: '', updated_at: '', created_by_id: null, updated_by_id: null };
        }
        else {
            const found = docs.find(d => d.id === b.document_id);
            if (!found)
                throw missing();
            d = found;
            if (!d.data.terms)
                invalid('document_id', 'This saved indent has no terms snapshot; apply the vendor terms migration before printing or dispatch');
            if (d.version !== b.version)
                throw repo.stale();
            before = structuredClone(d);
            if (b.action === 'print') {
                reason = 'Record vendor indent print request';
                d.data.print_requests.push({ requested_at: indiaInstant(), actor_id: a.id });
                if (d.data.sent_date === null)
                    await repo.notify(db, d.id, (await recipients(db, p)).ids, a.id);
            }
            else {
                reason = 'Record vendor indent dispatch';
                const today = indiaDate(new Date()), generated = indiaDate(new Date(d.data.generated_at));
                if (!b.sent_date || b.sent_date < generated || b.sent_date > today)
                    invalid('sent_date', 'Sent date must be between generation date and today');
                if (d.data.sent_date !== null)
                    invalid('sent_date', 'Dispatch is already recorded');
                const record = await groups.get(db, p.id, bo);
                if (!record)
                    throw missing();
                const command = groupInput({ action: 'invite', version: record.version, group_id: d.group_id, vendor_id: d.vendor_id, sent_date: b.sent_date, channel: b.channel, dispatch_reference: b.dispatch_reference, reason: 'Record vendor indent dispatch' });
                command.invitation_scope_revision = d.data.scope_revision;
                command.invitation_line_ids = d.data.lines.map(l => l.id);
                await groupCommand(db, a, p.id, bo, command);
                d.data.sent_date = b.sent_date;
                d.data.channel = b.channel;
                d.data.dispatch_reference = b.dispatch_reference;
                d.data.sent_recorded_at = indiaInstant();
                d.data.sent_by_id = a.id;
                await repo.resolve(db, d.id, a.id);
            }
            d.version++;
        }
        const saved = await repo.save(db, d, a.id);
        await repo.event(db, before, saved, b.action, reason, a.id);
        return workspaceBound(db, a, p, bo, write);
    });
}
export async function notifications(token: string) { return transaction(async (db) => { const a = await requireAnyPermission(db, token, ['projects.master.read', 'projects.purchase_indent_documents.read'], 'business'); return repo.notices(db, a.id); }); }
