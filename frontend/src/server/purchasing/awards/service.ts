import 'server-only';
import { randomUUID } from 'node:crypto';
import { transaction } from '../../db';
import { invalid, object, string, isoDate } from '../core/validation';
import { quantity } from '../core/decimal';
import { now } from '../core/time';
import { id } from '../approvals/validation';
import { members, source as workSource } from '../workflow/context';
import { assign, guard, stale } from '../workflow/policy';
import { text } from '../workflow/policy';
import { definitions } from '../workflow/catalogue';
import * as work from '../workflow/repository';
import { store as indents, save as saveIndent } from '../indents/repository';
import { lineSource, identities } from '../indents/source';
import { saveInput } from '../indents/validation';
import type { Release, Line } from '../indents/types';
import * as sourcing from '../sourcing/repository';
import * as sourcingContext from '../sourcing/context';
import { guard as sourceGuard, supplierGuard } from '../sourcing/policy';
import { allocation } from '../sourcing/values';
import type { Case, Offer } from '../sourcing/types';
import { indiaDate } from '../../personal-tasks/validation';
import * as context from './context';
import * as repo from './repository';
export async function workspace(token: string, p: string) { return transaction(async (db) => { id(p); const a = await context.authorize(db, token, p, false); let writable = true; try {
    await context.access(db, a, p, true);
}
catch {
    writable = false;
} const rows = []; for (const c of await context.candidates(db, p)) {
    const h = await repo.get(db, p, c.attempt.id), row = { loi_review_complete: false, id: c.attempt.id, bo_id: c.bo_id, group: c.attempt.group_name, vendor: c.attempt.vendor, reference: c.attempt.reference, currency: c.attempt.currency, total: c.attempt.total, items: c.lines.length, state: 'ready', packages: [] as repo.Package[] };
    if (h) {
        const docs = (await db.query('SELECT EXISTS(SELECT 1 FROM purchase_loi_reviews r WHERE r.document_id=d.id) AS reviewed FROM purchase_loi_documents d WHERE d.project_id=$1 AND d.attempt_id=$2 ORDER BY d.version DESC,d.id', [p, c.attempt.id])).rows;
        row.loi_review_complete = !!docs[0]?.reviewed;
        row.state = 'loi_pending';
        row.packages = structuredClone(h.packages);
        let all = !!row.packages.length;
        for (const pkg of row.packages) {
            const record = await sourcing.get(db, p, pkg.id);
            if (record) {
                pkg.state = record.state;
                pkg.loi_reference = record.loi_reference;
            }
            all = all && record?.state === 'released';
        }
        if (all)
            row.state = 'loi_released';
    }
    rows.push(row);
} return { rows, members: writable ? await members(db) : [], can_write: writable }; }); }
export async function execute(token: string, p: string, raw: Record<string, unknown>) {
    return transaction(async (db) => {
        id(p);
        const a = await context.authorize(db, token, p, true), q = object(raw, ['attempt_id', 'doer_id', 'resolver_id', 'planned_date', 'reason']), attempt = id(q.attempt_id), doer = id(q.doer_id), resolver = id(q.resolver_id), reason = text('reason', string(q.reason, 'reason', ''), 2000), planned = string(q.planned_date, 'planned_date', '');
        if (!isoDate(planned))
            invalid('planned_date', 'Choose a valid planned date');
        await db.query("SELECT pg_advisory_xact_lock(hashtextextended('purchase-award:'||$1::text,0))", [p]);
        const c = (await context.candidates(db, p)).find(c => c.attempt.id === attempt);
        if (!c)
            invalid('attempt_id', 'Use a current Finance-approved group; refresh changed evidence');
        const existing = await repo.get(db, p, attempt);
        if (existing)
            return repo.handoffView(existing);
        const approval = c.attempt;
        if (!approval.reviewed_by_id || !approval.reviewed_at || approval.reviewed_by_id === approval.submitted_by_id || !c.lines.length)
            invalid('approval', 'Independent Finance approval evidence is incomplete');
        const plan = Array.from({ length: 18 }, (_, i) => ({ number: i + 3, team: 'Procurement / project team', doer_id: doer, resolver_id: resolver, planned_date: planned })), at = now(), options = await members(db);
        assign({ steps: definitions() } as workPackage, plan, options, a.id, at);
        const all = await indents.list(db, p), indentID = randomUUID(), input = { currency: approval.currency, doer_id: doer, resolver_id: resolver, documents: 'Approved quotation ' + approval.reference + ' / ' + approval.quote_key, lines: [] as Line[] }, released: Release[] = [], versions: Record<string, number> = {};
        for (const l of c.lines) {
            const source = await lineSource(db, p, l.demand_id);
            if (source.item.id !== l.approval.item.id)
                throw stale();
            const line = { demand_id: l.demand_id, quantity: l.approval.quantity, budget: l.approval.budget ?? '0', full_kit: 'Vendor drawings, datasheets, QAP and applicable test certificates for ' + l.approval.item.description };
            let committed = BigInt(0);
            for (const other of all)
                for (const r of other.data.released)
                    if (r.demand_id === l.demand_id) {
                        committed += quantity(r.quantity);
                        if (committed > BigInt('9223372036854775807'))
                            invalid('quantity', 'Released quantity exceeds supported range');
                    }
            if (!source.active)
                invalid('demand_id', 'Project requirement is inactive');
            if (source.policy.status !== 'eligible')
                invalid('demand_id', 'Client make policy blocks release: ' + source.policy.reasons.join('; '));
            if (source.holds.holds.length)
                invalid('demand_id', 'Client purchase hold blocks release');
            const cap = quantity(source.quantity);
            if (committed > cap || quantity(line.quantity) > cap - committed)
                invalid('quantity', 'Quantity exceeds remaining project requirement');
            const release = { ...line, at, actor_id: a.id, evidence: 'Finance-approved award ' + approval.id, source };
            input.lines.push(line);
            versions[l.demand_id] = source.demand_version;
            released.push(release);
        }
        const validated = saveInput({ data: input, version: 1, reason });
        await identities(db, validated.data);
        const indent = await saveIndent(db, { id: indentID, project_id: p, version: 1, is_active: true, data: { ...validated.data, number: 'PI-' + indentID.replaceAll('-', '').slice(0, 12).toUpperCase(), state: 'released', released, review: { actor_id: approval.reviewed_by_id, at: approval.reviewed_at, evidence: 'Existing Finance approval ' + approval.id, demand_versions: versions } } }, a.id);
        await indents.event(db, null, indent, 'approved_award_handoff', reason, a.id);
        const h: repo.Handoff = { id: randomUUID(), project_id: p, bo_id: c.bo_id, attempt_id: approval.id, created_by: a.id, created_at: at, packages: [] };
        for (const [i, l] of c.lines.entries()) {
            const current = await workSource(db, p, indent.id, l.demand_id);
            guard(current);
            const pkgID = randomUUID(), pkg: workPackage = assign({ id: pkgID, project_id: p, indent_id: indent.id, demand_id: l.demand_id, code: 'PF-' + pkgID, description: current.description, source_snapshot: current, version: 1, is_active: true, created_at: at, updated_at: at, created_by_id: a.id, updated_by_id: a.id, steps: definitions() }, plan, options, a.id, at);
            for (const s of pkg.steps)
                if (s.number <= 2)
                    s.state = 'source_linked';
            const savedPackage = await work.save(db, pkg, a.id);
            await work.event(db, null, savedPackage, 'create', reason, a.id, savedPackage.updated_at);
            const src = await sourcingContext.source(db, p, pkgID);
            sourceGuard(src);
            const supplier = (await sourcingContext.suppliers(db, p, pkgID)).find(s => s.id === approval.vendor_id);
            supplierGuard(supplier, true);
            const price = l.price, offer: Offer = { id: randomUUID(), supplier_id: approval.vendor_id, supplier_snapshot: supplier, due_date: '', rfq_terms: '', state: 'quoted', sent_date: '', channel: '', dispatch_reference: '', quote_reference: approval.reference, evidence_reference: approval.quote_key, valid_until: approval.valid_until ?? '', pricing_mode: price.pricing_mode, ...(price.pricing_mode === 'line_total' ? { line_total: price.base } : {}), unit_price: price.pricing_mode === 'line_total' ? '' : price.unit_price, discount: price.discount, tax: price.tax, freight: price.allocated_charges, other: '0', delivery_days: l.delivery_days, payment_terms: l.payment_terms, warranty: l.warranty, compliant: true, compliance_notes: 'Existing approved quotation selection', no_bid_reason: '', is_active: true, created_at: at, updated_at: at, created_by_id: a.id, updated_by_id: a.id };
            const proof = await sourcingContext.verify(db, p, pkgID, offer);
            if ((approval.valid_until ?? '') < indiaDate(new Date(at)))
                invalid('valid_until', 'Approved quotation expired; revise it before award');
            const record: Case = { award_reference: approval.id, id: randomUUID(), project_id: p, package_id: pkgID, source_snapshot: src, approver_id: approval.reviewed_by_id, version: 1, state: 'draft', offers: [offer], is_active: true, created_at: at, updated_at: at, created_by_id: a.id, updated_by_id: a.id, selected_offer_id: '', selection_reason: '', loi_terms: '', proposed_by_id: '', approved_by_id: '', approved_at: null, decision_snapshot: null, loi_reference: '', loi_sent_date: '', loi_channel: '', loi_dispatch_reference: '', released_at: null };
            const before = await sourcing.save(db, record, a.id);
            await sourcing.event(db, null, before, 'configure', 'Prepare existing Finance-approved award ' + approval.id, a.id);
            record.state = 'approved';
            record.version++;
            record.selected_offer_id = offer.id;
            record.selection_reason = approval.reason;
            record.loi_terms = approval.terms ?? '';
            record.proposed_by_id = approval.submitted_by_id;
            record.approved_by_id = approval.reviewed_by_id;
            record.approved_at = approval.reviewed_at;
            record.decision_snapshot = { source: src, offer, terms: record.loi_terms, reason: record.selection_reason, financial_reference: proof.reference + ':' + proof.fingerprint, comparison: [{ offer_id: offer.id, total: l.approval.quoted, rank: null, expired: false }] };
            const used = await sourcing.balances(db, p, l.demand_id, record.id, record.decision_snapshot.financial_reference), debit = allocation(src.quantity, proof.quantity, proof.total, used.quantity, used.amount, l.approval.quoted), saved = await sourcing.save(db, record, a.id);
            await sourcing.reserve(db, saved, proof, debit);
            await sourcing.event(db, before, saved, 'award', reason, a.id);
            h.packages.push({ id: pkgID, description: released[i].source.item.description, quantity: src.quantity, uom: released[i].source.item.uom, state: 'approved', loi_reference: '' });
        }
        await repo.save(db, h, reason);
        return repo.handoffView(h);
    });
}
import type { Package as workPackage } from '../workflow/types';
