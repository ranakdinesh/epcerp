import type { DB } from '../../db';
import { receiptState, source as quotationsSource, type QuotationInputs } from '../quotations/source';
import { get as quotationRecord, revisions } from '../quotations/repository';
import { compare } from '../quotations/calculation';
import { get as groups } from '../groups/repository';
import { indiaDate } from '../../personal-tasks/validation';
import { effectiveApproved } from '../../projects/budgets/approved';
import type { Source } from './types';
export async function source(db: DB, project: string, bo: string, inputs?: QuotationInputs & { quotation: Awaited<ReturnType<typeof quotationRecord>> }): Promise<Source> {
    const current = await quotationsSource(db, project, bo, inputs), grouping = inputs ? inputs.groups : await groups(db, project, bo), quoted = compare(await revisions(db, project, bo), current.documents, indiaDate(new Date()), current.currency);
    const budget = await effectiveApproved(db, project);
    const quotes: Source['quotes'] = [];
    for (const q of quoted) {
        quotes.push({ lowest: q.suggested, terms: `Delivery: ${q.delivery_days} days; ${q.delivery_terms}. Payment: ${q.payment_terms}${q.amount_mode === 'total_only' ? ' · Lump-sum offer with internal tax-inclusive item allocations' : ''}`, valid_until: q.valid_until, key: q.document_id + ':' + q.revision, document_id: q.document_id, group_id: q.group_id, group_name: q.group_name, vendor_id: q.vendor_id, vendor: q.vendor, reference: q.reference, total: q.total, revision: q.revision, scope_revision: q.scope_revision, eligible: q.eligible, exclusions: [...q.exclusions], lines: q.lines.map(l => ({ coverage: l.coverage_type, months: l.warranty_months, notes: l.notes, price: { discount_percent: l.discount_pct, tax_percent: l.tax_pct, pricing_mode: l.pricing_mode || 'unit_price', unit_price: l.unit_price ?? '', base: l.base ?? '', discount: l.discount_amount ?? '', tax: l.tax_amount ?? '', allocated_charges: l.allocated_charges ?? '' }, line_id: l.line_id, quantity: l.quantity, quoted: l.landed ?? '', item: { id: l.source.item_snapshot.id, sku: l.source.item_snapshot.sku, description: l.source.item_snapshot.description, uom: l.source.item_snapshot.uom, manufacturer: l.source.item_snapshot.manufacturer, engineering_key: l.source.item_snapshot.engineering_key } })) });
    }
    const receipt = receiptState(current.documents, inputs ? inputs.quotation : await quotationRecord(db, project, bo), current.expectedReceipts);
    return { receipt_frozen: current.dispatchComplete && receipt.frozen, project_id: project, bo_id: bo, currency: current.currency, available: current.available, groups: grouping?.is_active ? grouping.groups.map(g => ({ id: g.id, name: g.name, scope_revision: g.scope_revision })) : [], quotes, budget };
}
