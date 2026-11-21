import type { DB } from '../../db';
import { project, open } from '../core/access';
import { get as getBO } from '../bo/repository';
import { list as indentDocuments } from '../indent-documents/repository';
import { get as grouping } from '../groups/repository';
import { purchaseVendors } from '../../master/vendors/purchase-reader';
import type { Document, QuoteRecord } from './types';
import { PurchaseError } from '../core/validation';
export type QuotationInputs = {
    list: NonNullable<Awaited<ReturnType<typeof getBO>>>;
    groups: Awaited<ReturnType<typeof grouping>>;
    documents: Awaited<ReturnType<typeof indentDocuments>>;
};
export async function source(db: DB, target: string, bo: string, inputs?: QuotationInputs) {
    const b = inputs ? inputs.list : await getBO(db, target, bo);
    if (!b?.is_active)
        throw new PurchaseError(404, 'Quotation or evidence not found in this project BO');
    const p = await project(db, target);
    const settings = (await db.query<{ currency: string; client_active: boolean }>(
        'SELECT p.currency, c.is_active AS client_active FROM projects p LEFT JOIN clients c ON c.id=p.client_id WHERE p.id=$1',
        [target],
    )).rows[0];
    const groups = inputs ? inputs.groups : await grouping(db, target, bo);
    const rev = new Map(groups?.is_active ? groups.groups.map(g => [g.id, g]) : []);
    const docs = (inputs ? inputs.documents : await indentDocuments(db, target, bo)).filter(doc => doc.is_active);
    const vendorIds = new Set(docs.length ? (await purchaseVendors(db)).map(vendor => vendor.id) : []);
    const documents: Document[] = [];
    for (const d of docs) {
        const group = rev.get(d.group_id);
        const data: Document['data'] = { number: d.data.number, group_name: d.data.group_name, scope_revision: d.data.scope_revision, is_current: d.data.scope_revision === group?.scope_revision && group.vendor_ids.includes(d.vendor_id), sent_date: d.data.sent_date, vendor: d.data.vendor, lines: d.data.lines };
        if (d.data.type_indent) data.indent_category = d.data.type_indent.category;
        if (d.data.terms)
            data.terms = d.data.terms;
        documents.push({ id: d.id, project_id: d.project_id, bo_id: d.bo_id, group_id: d.group_id, vendor_id: d.vendor_id, version: d.version, vendor_available: vendorIds.has(d.vendor_id), data });
    }
    const expectedReceipts = [...rev.values()].reduce((count, group) => count + group.vendor_ids.length, 0);
    return { documents, expectedReceipts, dispatchComplete: b.state === 'completed' && dispatchComplete(documents, expectedReceipts), currency: settings.currency, available: open(p) && p.purchase_enabled && !!settings.client_active };
}


type ReceiptDocument = {id: string; data: Pick<Document['data'], 'is_current' | 'sent_date'>};

function dispatchComplete(documents: ReceiptDocument[], expected: number) {
    const current = documents.filter(document => document.data.is_current);
    return expected > 0 && current.length === expected && current.every(document => Boolean(document.data.sent_date));
}

export function receiptState(documents: ReceiptDocument[], record: QuoteRecord | null, expectedReceipts: number) {
    const current = documents.filter(document => document.data.is_current);
    const documentIds = current.map(document => document.id).sort();
    const quoted = new Set(record?.is_active ? record.quotes.map(quote => quote.document_id) : []);
    const ready = dispatchComplete(documents, expectedReceipts) && current.every(document => quoted.has(document.id));
    const snapshot = record?.frozen_document_ids;
    const frozen = ready && snapshot?.length === documentIds.length && documentIds.every((id, index) => snapshot[index] === id);
    return { documentIds, ready, frozen };
}
