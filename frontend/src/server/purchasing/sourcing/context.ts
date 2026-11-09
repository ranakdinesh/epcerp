import type { DB } from '../../db';
import { requireAnyPermission, assignmentIdentities } from '../../auth/access';
import { AppError } from '../../errors';
import { project, scope, type Actor } from '../core/access';
import { projectAccess } from '../core/project-access';
import { workspace } from '../workspace/repository';
import * as work from '../workflow/reader';
import { actions as workActions } from '../workflow/context';
import { store as indents } from '../indents/repository';
import { purchaseTaskItem } from '../../master/items/task-reader';
import { purchaseVendors } from '../../master/vendors/purchase-reader';
import { demandLink } from '../bo/repository';
import { approved, commercial } from '../approvals/service';
import { fingerprint } from '../approvals/fingerprint';
import { invalid, trim } from '../core/validation';
import { missing } from './policy';
import { scopePrice, total, priceOffer } from './values';
import type { Source, Capabilities, Supplier, Proof, Offer } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_sourcing.read'], 'business');
export async function scoped(db: DB, a: Actor, p: string, code: string, write: boolean) { const parent = await project(db, p, true), w = await workspace(db, p); try {
    const v = await scope(db, a, parent, w, code, write);
    return !write || v.can_write;
}
catch (e) {
    if (!(e instanceof AppError) || e.status !== 403)
        throw e;
    try {
        const v = await projectAccess(db, a, parent, code, write);
        return !write || v.can_write;
    }
    catch (e) {
        if (e instanceof AppError && e.status === 403)
            return false;
        throw e;
    }
} }
export async function access(db: DB, a: Actor, p: string, id: string): Promise<Capabilities> { await project(db, p, true); const caps: Capabilities = { read: false, write: false, approve: false, configure: false, budget_read: false, scoped: false, cancel: false, work: {} }; if (!a.allows('projects.purchase_sourcing.read'))
    return caps; caps.scoped = await scoped(db, a, p, 'projects.purchase_sourcing.read', false); caps.read = caps.scoped; for (const n of [3, 4, 5]) {
    const value = await work.stepAccess(db, a, p, id, n);
    caps.work[n] = value.work;
    caps.read ||= value.read;
    if (n === 4)
        caps.cancel = value.cancel;
} caps.write = caps.read && a.allows('projects.purchase_sourcing.write'); caps.cancel = caps.write && caps.cancel; if (caps.scoped && a.allows('projects.purchase_sourcing.approve'))
    try {
        caps.approve = await scoped(db, a, p, 'projects.purchase_sourcing.approve', true);
    }
    catch (e) {
        if (!(e instanceof AppError) || e.status !== 422)
            throw e;
    } if (caps.scoped && caps.write)
    caps.configure = (await workActions(db, a, p)).write; return caps; }
export async function source(db: DB, p: string, id: string): Promise<Source> { const { record, current } = await work.inspect(db, p, id), indent = await indents.get(db, p, record.indent_id); if (!indent)
    throw missing(); const blockers = [...current.blockers]; if (!current.project_available)
    blockers.push('Project is inactive, held, closed or purchasing is disabled'); for (const s of record.steps)
    if (s.number >= 3 && s.number <= 5 && s.state === 'blocked')
        blockers.push('Sourcing preparation has an unresolved blocker'); return { package_id: id, demand_id: record.demand_id, demand_version: indent.data.released.find(r => r.demand_id === record.demand_id)?.source.demand_version ?? 0, quantity: current.quantity, currency: indent.data.currency, description: current.description, project_available: current.project_available, eligible: current.eligible, blockers }; }
export async function approvers(db: DB, p: string) { const out = []; for (const a of await assignmentIdentities(db, [])) {
    if (!a.allows('projects.purchase_sourcing.read') || !a.allows('projects.purchase_sourcing.approve'))
        continue;
    if (await scoped(db, a, p, 'projects.purchase_sourcing.read', false) && await scoped(db, a, p, 'projects.purchase_sourcing.approve', false))
        out.push({ id: a.id, label: a.name });
} return out; }
export async function suppliers(db: DB, p: string, id: string): Promise<Supplier[]> {
    const { record } = await work.inspect(db, p, id, true), indent = await indents.get(db, p, record.indent_id);
    if (!indent)
        throw missing();
    const itemID = indent.data.released.find(r => r.demand_id === record.demand_id)?.source.item.id;
    if (!itemID)
        invalid('source', 'Released material item evidence is missing');
    const item = await purchaseTaskItem(db, itemID);
    if (!item.approved_for_purchase)
        invalid('item_id', 'Select an active approved catalogue item with current configuration and manufacturer eligibility');
    const link = await demandLink(db, p, record.demand_id), allowed = new Set<string>();
    if (link?.item_id === item.id)
        for (const a of await approved(db, p, link.bo_id, false))
            if (a.lines.some(l => l.line_id === link.line_id && l.item.id === item.id))
                allowed.add(a.vendor_id);
    return (await purchaseVendors(db)).map(v => { const s = (key: string) => typeof v.data[key] === 'string' ? v.data[key] as string : '', contains = (key: string) => Array.isArray(v.data[key]) && (v.data[key] as unknown[]).includes(item.id), category = Array.isArray(v.data.category_ids) && (v.data.category_ids as unknown[]).includes(item.category_id); return { id: v.id, name: v.name, code: v.code, version: v.version, eligible: v.is_active && s('qualification') === 'approved' && (allowed.has(v.id) || contains('item_ids') || category), registration: s('registration_reference'), contact: trim(s('contact')) ? s('contact') : trim(s('phone')) ? s('phone') : s('email'), location: s('location') }; });
}
export async function verify(db: DB, p: string, id: string, o: Offer): Promise<Proof> { const { record } = await work.inspect(db, p, id, true), link = await demandLink(db, p, record.demand_id); if (!link)
    invalid('financial_approval', 'Current source has no completed BO linkage'); const attempts = await approved(db, p, link.bo_id, false), prices = await commercial(db, p, link.bo_id, false); for (const a of attempts) {
    if (a.vendor_id !== o.supplier_id || a.reference !== o.quote_reference || a.quote_key !== o.evidence_reference)
        continue;
    const line = a.lines.find(l => l.line_id === link.line_id && l.item.id === link.item_id);
    if (!line)
        continue;
    const price = prices.find(v => v.attempt_id === a.id && v.line_id === link.line_id && v.item_id === link.item_id && v.vendor_id === a.vendor_id && v.quote_key === a.quote_key)?.price;
    if (!price)
        invalid('financial_approval', 'Exact current approved price evidence is missing');
    return { price, reference: a.id, fingerprint: fingerprint(a), vendor_id: a.vendor_id, quantity: line.quantity, total: line.quoted, currency: a.currency, effective: true, quote_reference: a.reference, evidence_reference: a.quote_key };
} invalid('financial_approval', 'Select current Finance-approved evidence for this exact BO item and vendor'); }
export async function evidence(db: DB, p: string, id: string) { const { record, current } = await work.inspect(db, p, id, true), link = await demandLink(db, p, record.demand_id); if (!link)
    return []; let attempts; try {
    attempts = await approved(db, p, link.bo_id, false);
}
catch (e) {
    if (e instanceof AppError && e.status === 422)
        return [];
    throw e;
} const prices = await commercial(db, p, link.bo_id, false), out = []; for (const a of attempts)
    for (const line of a.lines) {
        if (line.line_id !== link.line_id || line.item.id !== link.item_id)
            continue;
        const raw = prices.find(v => v.attempt_id === a.id && v.line_id === link.line_id && v.item_id === link.item_id)?.price, price = raw ? scopePrice(raw, current.quantity, line.quantity) : null;
        out.push({ scoped_quantity: current.quantity, scoped_total: price ? total(priceOffer(price), current.quantity) : '', ...(price ? { price } : {}), id: a.id, vendor_id: a.vendor_id, quote_reference: a.reference, evidence_reference: a.quote_key, quantity: line.quantity, currency: a.currency, total: line.quoted });
    } return out; }
