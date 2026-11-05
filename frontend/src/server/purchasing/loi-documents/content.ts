import { createHash } from 'node:crypto';
import type { DB } from '../../db';
import { goJSON } from '../../master/catalogue/serialization';
import { purchaseTaskItem } from '../../master/items/task-reader';
import { invalid } from '../core/validation';
import { format } from '../core/decimal';
import { money } from '../approvals/policy';
import { purchaseVendors } from '../../master/vendors/purchase-reader';
import { indiaDate } from '../../personal-tasks/validation';
import { project } from '../core/access';
import { candidates } from '../awards/context';
import { get as handoff } from '../awards/repository';
import { get as sourcing } from '../sourcing/repository';
import { source, suppliers, verify } from '../sourcing/context';
import { guard, selected, supplierGuard } from '../sourcing/policy';
import { stale, missing } from '../workflow/policy';
import { fingerprint } from '../approvals/fingerprint';
import type { Content, Command } from './types';
export async function content(db: DB, p: string, attempt: string, q: Command): Promise<Content> { const h = await handoff(db, p, attempt); if (!h)
    invalid('award', 'Prepare the approved award before generating its LOI'); const c = (await candidates(db, p)).find(c => c.attempt.id === attempt); if (!c)
    invalid('approval', 'Refresh the current Finance approval and supplier documents before generating this LOI'); const a = c.attempt; if (!a.reviewed_at || q.date < indiaDate(new Date(a.reviewed_at)))
    invalid('date', 'LOI date must be on or after Finance approval'); const parent = await project(db, p, false), client = (await db.query('SELECT display_name AS name FROM clients WHERE id=$1', [parent.client_id])).rows[0], vendor = (await purchaseVendors(db)).find(v => v.id === a.vendor_id); if (!vendor)
    throw missing(); const data = vendor.data ?? {}, str = (v: unknown) => typeof v === 'string' ? v : '', out: Content = { ...q, TemplateVersion: 4, Project: parent.name, ProjectNumber: parent.project_number, Client: client?.name ?? '', Group: a.group_name, Vendor: vendor.name, VendorCode: vendor.code, Address: str(data.address) || str(data.location), ContactPerson: str(data.contact_person) || str(data.contact_person_name), Phone: str(data.phone), Email: str(data.email) || str(data.contact), GST: str(data.gst_number), Quote: a.reference, Currency: a.currency, Total: a.total, ApprovedTerms: a.terms ?? '', SourceHash: fingerprint(a), Items: [] }; for (const l of c.lines) {
    let record = null;
    for (const pkg of h.packages) {
        const r = await sourcing(db, p, pkg.id);
        if (r?.source_snapshot.demand_id === l.demand_id)
            record = r;
    }
    if (!record || record.award_reference !== attempt || !['approved', 'released'].includes(record.state))
        invalid('award', 'Current approved item handoff is unavailable');
    const s = await source(db, p, record.package_id);
    guard(s);
    if (s.demand_version !== record.source_snapshot.demand_version || s.quantity !== record.source_snapshot.quantity)
        throw stale();
    const offer = selected(record, indiaDate(new Date())), supplier = (await suppliers(db, p, record.package_id)).find(s => s.id === a.vendor_id);
    supplierGuard(supplier, true);
    if (offer.supplier_id !== a.vendor_id || supplier.version !== offer.supplier_snapshot.version)
        throw stale();
    const proof = await verify(db, p, record.package_id, offer);
    if (!proof?.effective || record.decision_snapshot?.financial_reference !== proof.reference + ':' + proof.fingerprint)
        throw stale();
    const item = l.approval.item, actual = await purchaseTaskItem(db, item.id), lines: string[] = [];
    for (const field of actual.snapshot.configuration.fields) {
        let value = actual.specifications[field.code];
        if (!value)
            continue;
        value = field.options.find(o => o.code === value)?.label ?? value;
        lines.push(field.label + ': ' + value);
    }
    if (actual.model)
        lines.push('Model: ' + actual.model);
    out.Items.push({ Description: item.description, SKU: item.sku, Make: item.manufacturer, UOM: item.uom, Quantity: l.approval.quantity, Price: l.price, Total: l.approval.quoted, Payment: l.payment_terms, Warranty: l.warranty, Technical: lines.join('\n'), DeliveryDays: l.delivery_days });
} totals(out); return out; }
export function totals(c: Content) { if (!c.Items.length)
    invalid('items', 'LOI requires approved items'); const sums = { base: BigInt(0), discount: BigInt(0), tax: BigInt(0), charges: BigInt(0), total: BigInt(0) }; for (const i of c.Items) {
    const values = { base: money(i.Price.base), discount: money(i.Price.discount), tax: money(i.Price.tax), charges: money(i.Price.allocated_charges), total: money(i.Total) };
    if (values.base - values.discount + values.tax + values.charges !== values.total)
        invalid('price', 'Approved LOI item components do not reconcile');
    for (const k of Object.keys(sums) as (keyof typeof sums)[])
        sums[k] += values[k];
} if (money(c.Total) !== sums.total)
    invalid('total', 'Approved LOI total does not reconcile'); return Object.fromEntries(Object.entries(sums).map(([k, v]) => [k, format(v, 2)])) as Record<keyof typeof sums, string>; }
export function digest(c: Content) { const canonical = { reference: c.reference, date: c.date, format: c.format, notes: c.notes, TemplateVersion: c.TemplateVersion, Project: c.Project, ProjectNumber: c.ProjectNumber, Client: c.Client, Group: c.Group, Vendor: c.Vendor, VendorCode: c.VendorCode, Address: c.Address, ContactPerson: c.ContactPerson, Phone: c.Phone, Email: c.Email, GST: c.GST, Quote: c.Quote, Currency: c.Currency, Total: c.Total, ApprovedTerms: c.ApprovedTerms, SourceHash: c.SourceHash, Items: c.Items.map(i => ({ Description: i.Description, SKU: i.SKU, Make: i.Make, UOM: i.UOM, Quantity: i.Quantity, Price: { discount_percent: i.Price.discount_percent, tax_percent: i.Price.tax_percent, pricing_mode: i.Price.pricing_mode, unit_price: i.Price.unit_price, base: i.Price.base, discount: i.Price.discount, tax: i.Price.tax, allocated_charges: i.Price.allocated_charges }, Total: i.Total, Payment: i.Payment, Warranty: i.Warranty, Technical: i.Technical, DeliveryDays: i.DeliveryDays })) }; return createHash('sha256').update(goJSON(canonical)).digest('hex'); }
