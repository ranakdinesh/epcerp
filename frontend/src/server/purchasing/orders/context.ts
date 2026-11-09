import type { DB } from '../../db';
import { requireAnyPermission } from '../../auth/access';
import { AppError } from '../../errors';
import { project, type Actor } from '../core/access';
import { inspect, stepAccess } from '../workflow/reader';
import { scoped } from '../sourcing/context';
import { currentReleasedFact } from '../sourcing/reader';
import { clearance } from '../technical/service';
import { purchaseVendors } from '../../master/vendors/purchase-reader';
import { VerificationRepository } from '../../master/verification/repository';
import { evaluate } from '../../master/verification/domain';
import { store as indents } from '../indents/repository';
import { indiaDate } from '../../personal-tasks/validation';
import { basicAmount } from './policy';
import type { Source, Capabilities } from './types';
export const actor = (db: DB, token: string) => requireAnyPermission(db, token, ['projects.purchase_orders.read'], 'business');
export async function access(db: DB, a: Actor, p: string, id: string): Promise<Capabilities> { await project(db, p, true); const c: Capabilities = { read: false, write: false, release: false, scoped: false, work: false, record_client: false }; if (!a.allows('projects.purchase_orders.read'))
    return c; c.scoped = await scoped(db, a, p, 'projects.purchase_orders.read', false); const task = await stepAccess(db, a, p, id, 9); c.read = c.scoped || task.read; c.work = task.work; c.write = c.read && a.allows('projects.purchase_orders.write'); c.release = c.write && c.work && a.allows('projects.purchase_orders.release'); c.record_client = c.write && c.work && a.allows('projects.purchase_orders.client_approval'); return c; }
const blank = (p: string, id: string): Source => ({ project_id: p, package_id: id, project_name: '', site: '', sourcing_id: '', sourcing_version: 0, loi_reference: '', loi_date: '', financial_reference: '', supplier_verification: '', supplier: { id: '', name: '', code: '', address: '', contact: '', email: '', phone: '', tax_reference: '', registration: '' }, technical: { case_id: '', version: 0, revision: 0, reference: '', approval_date: '', approval_file_id: '', transmittal_reference: '', qap_approved: false, conditions_closed: false }, item_id: '', sku: '', description: '', uom: '', quantity: '', currency: '', pricing_mode: '', unit_price: '', base: '', discount: '', tax: '', freight: '', other: '', total: '', delivery_days: 0, payment_terms: '', warranty: '', loi_terms: '', available: false, blockers: [] });
const block = (s: Source, message: string) => { s.available = false; s.blockers.push(message); return s; };
const failure = (s: Source, e: unknown) => { if (e instanceof AppError && e.status === 422)
    return block(s, e.message); throw e; };
export async function source(db: DB, p: string, id: string): Promise<Source> {
    const { record, current } = await inspect(db, p, id), out: Source = { ...blank(p, id), project_name: current.project_name, site: current.site, description: current.description, quantity: current.quantity, uom: current.uom, available: current.project_available && current.eligible, blockers: [...current.blockers] };
    for (const s of record.steps)
        if (s.number === 9 && s.state === 'blocked')
            out.blockers.push('Purchase-order release has an unresolved blocker');
    let loi;
    try {
        loi = await currentReleasedFact(db, p, id);
    }
    catch (e) {
        return failure(out, e);
    }
    if (!loi)
        return block(out, 'Release a current independently approved LOI before preparing the order');
    const offer = loi.offer;
    Object.assign(out, { sourcing_id: loi.case_id, sourcing_version: loi.version, loi_reference: loi.loi_reference, loi_date: indiaDate(new Date(loi.released_at)), financial_reference: loi.financial_reference, currency: loi.source.currency, quantity: loi.source.quantity, description: loi.source.description, total: loi.total, pricing_mode: offer.pricing_mode || 'unit_price', unit_price: offer.unit_price, discount: offer.discount, tax: offer.tax, freight: offer.freight, other: offer.other, delivery_days: offer.delivery_days, payment_terms: offer.payment_terms, warranty: offer.warranty, loi_terms: loi.loi_terms });
    out.base = basicAmount(out.pricing_mode, offer.unit_price, offer.line_total ?? '', out.quantity);
    const supplier = (await purchaseVendors(db)).find(v => v.id === offer.supplier_id && v.is_active);
    if (!supplier)
        return block(out, 'The selected supplier is no longer active');
    const field = (key: string) => typeof supplier.data[key] === 'string' ? supplier.data[key] as string : '';
    out.supplier = { id: supplier.id, name: supplier.name, code: supplier.code, address: field('location'), contact: field('contact'), email: field('email'), phone: field('phone'), tax_reference: field('gst_number'), registration: field('registration_reference') };
    const repo = new VerificationRepository(db), verified = evaluate(await repo.snapshot(supplier.id), await repo.review(supplier.id));
    if (!verified.can_generate_purchase_order)
        return block(out, verified.message);
    out.supplier_verification = verified.fingerprint;
    let technical;
    try {
        technical = await clearance(db, p, id);
    }
    catch (e) {
        return failure(out, e);
    }
    if (!technical)
        return block(out, 'Complete the client-approved technical kit and signed QAP before preparing the PO');
    const src = technical.source;
    if (technical.project_id !== p || technical.package_id !== id || src.case_id !== loi.case_id || src.version !== loi.version || src.supplier_id !== offer.supplier_id || src.financial_reference !== loi.financial_reference || src.quantity !== loi.source.quantity || src.total !== loi.total)
        return block(out, 'Technical clearance does not match the exact current commercial LOI');
    out.technical = { transmittal_reference: technical.transmittal_reference, case_id: technical.case_id, version: technical.version, revision: technical.revision, reference: technical.approval.reference, approval_date: technical.approval.date, approval_file_id: technical.approval.file_id, qap_approved: technical.approval.qap_approved, conditions_closed: technical.approval.status === 'approved' || technical.approval.conditions_closed };
    const indent = await indents.get(db, p, record.indent_id);
    if (!indent)
        return block(out, 'Released item evidence is unavailable');
    for (const l of indent.data.released)
        if (l.demand_id === record.demand_id) {
            out.item_id = l.source.item.id;
            out.sku = l.source.item.sku;
            out.uom = l.source.item.uom;
        }
    return out;
}
