import { isDeepStrictEqual } from 'node:util';
import { PurchaseError, invalid, trim } from '../core/validation';
import type { Actor } from '../core/access';
import { indiaDate } from '../../personal-tasks/validation';
import type { Source, Supplier, Case, Offer, Capabilities, Proof, Member, Command } from './types';
import { id, total, scopePrice, priceOffer, mode, amount, rate, scaled, comparisons } from './values';
import { validate } from './validation';
export const forbidden = () => new PurchaseError(403, 'Current sourcing authority or assigned task is required');
export const missing = () => new PurchaseError(404, 'Sourcing case or offer not found');
export const stale = () => new PurchaseError(409, 'Sourcing or its source changed; reload the saved version');
export function guard(s: Source) { id(s.package_id); id(s.demand_id); if (s.demand_version < 1 || !/^[A-Z]{3}$/.test(s.currency))
    invalid('source', 'Use a current source revision and currency'); if (!s.project_available || !s.eligible || s.blockers.length)
    invalid('source', 'Current source controls block sourcing: ' + s.blockers.join('; ')); scaled('quantity', s.quantity, 4, 19, true); }
export function supplierGuard(s: Supplier | undefined, onboarding = false): asserts s is Supplier { if (!s?.eligible)
    invalid('supplier_id', 'Supplier must be active, qualified and linked to this item'); if (onboarding && (!trim(s.registration) || !trim(s.contact) || !trim(s.location)))
    invalid('supplier_id', 'Complete supplier registration, contact and location before LOI release'); }
export function selected(c: Case, today: string) { const o = c.offers.find(o => o.id === c.selected_offer_id); if (!o)
    throw missing(); if (o.state !== 'quoted' || !o.compliant)
    invalid('offer_id', 'Select a received technically compliant quotation'); if (!o.valid_until || o.valid_until < today)
    invalid('valid_until', 'Selected quotation expired; return to draft and revise it'); return o; }
export const sameSource = (a: Source, b: Source) => a.package_id === b.package_id && a.demand_id === b.demand_id && a.demand_version === b.demand_version && a.quantity === b.quantity && a.currency === b.currency;
export function accessGuard(a: Actor, caps: Capabilities, c: Case | null, action: string) { if (!a.active || !a.id)
    throw new PurchaseError(401, 'Authentication required'); if (!caps.read)
    throw forbidden(); if (action === 'decide') {
    if (!caps.approve || !c || a.id !== c.approver_id || a.id === c.proposed_by_id || c.state !== 'pending')
        throw forbidden();
    return;
} if (!caps.write)
    throw forbidden(); if (action === 'reopen' && caps.cancel)
    return; if (action === 'configure') {
    if (!caps.configure)
        throw forbidden();
    return;
} const n = ['rfq', 'dispatch', 'no_bid'].includes(action) ? 3 : action === 'release' ? 5 : 4; if (!caps.work[n])
    throw forbidden(); }
export function proofGuard(c: Case, o: Offer, p: Proof | null): asserts p is Proof {
    if (!p?.effective || !p.reference || !p.fingerprint || p.vendor_id !== o.supplier_id || p.currency !== c.source_snapshot.currency || !p.quote_reference || !p.evidence_reference || p.quote_reference !== o.quote_reference || p.evidence_reference !== o.evidence_reference)
        invalid('financial_approval', 'A current matching Finance-approved quotation is required');
    if (!p.price)
        invalid('financial_approval', 'Current approved commercial price evidence is required');
    try {
        if (total(priceOffer(p.price), p.quantity) !== amount('financial_approval', p.total))
            throw new Error('Inconsistent price');
    }
    catch {
        invalid('financial_approval', 'Approved quantity, price and total evidence must agree');
    }
    const price = scopePrice(p.price, c.source_snapshot.quantity, p.quantity), m = mode(o.pricing_mode ?? '');
    if (m !== price.pricing_mode)
        invalid('pricing_mode', 'Pricing mode differs from the approved quotation');
    if (m === 'line_total') {
        try {
            if (amount('line_total', o.line_total ?? '') !== amount('line_total', price.base))
                throw new Error('Different base');
        }
        catch {
            invalid('line_total', 'Line total differs from the approved quotation');
        }
    }
    else {
        const actual = rate(o.unit_price);
        try {
            if (actual !== rate(price.unit_price))
                throw new Error('Different rate');
        }
        catch {
            invalid('unit_price', 'Unit rate differs from the approved quotation');
        }
    }
    for (const [field, actual, expected] of [['discount', o.discount, price.discount], ['tax', o.tax, price.tax]] as const) {
        const normalized = amount(field, actual);
        try {
            if (normalized !== amount(field, expected))
                throw new Error('Different amount');
        }
        catch {
            invalid(field, 'Amount differs from the approved quotation');
        }
    }
    const freight = scaled('freight', o.freight, 2, 16), other = scaled('other', o.other, 2, 16);
    try {
        if (freight + other !== scaled('charges', price.allocated_charges, 2, 16))
            throw new Error('Different charges');
    }
    catch {
        invalid('freight', 'Allocated charges differ from the approved quotation');
    }
    const actual = total(o, c.source_snapshot.quantity);
    try {
        if (actual !== total(priceOffer(price), c.source_snapshot.quantity))
            throw new Error('Different total');
    }
    catch {
        invalid('financial_approval', 'Finance approval must match the scoped selected quotation amount');
    }
}
const blankOffer = (id: string, supplier: string, actor: string, at: string): Offer => ({ id, supplier_id: supplier, supplier_snapshot: { id: '', name: '', code: '', version: 0, eligible: false, registration: '', contact: '', location: '' }, due_date: '', rfq_terms: '', state: 'draft', sent_date: '', channel: '', dispatch_reference: '', quote_reference: '', evidence_reference: '', valid_until: '', unit_price: '', discount: '', tax: '', freight: '', other: '', delivery_days: 0, payment_terms: '', warranty: '', compliant: false, compliance_notes: '', no_bid_reason: '', is_active: true, created_at: at, created_by_id: actor, updated_at: '0001-01-01T00:00:00Z', updated_by_id: '' });
export function offerCommand(c: Case, q: Command, suppliers: Supplier[], actor: string, at: string, newID: string) {
    const today = indiaDate(new Date(at));
    let i = c.offers.findIndex(o => o.id === q.offer_id);
    if (q.action === 'rfq') {
        const s = suppliers.find(s => s.id === q.supplier_id);
        supplierGuard(s);
        if (q.due_date < today)
            invalid('due_date', 'RFQ due date must be today or later');
        if (q.offer_id) {
            if (i < 0 || c.offers[i].state !== 'draft' || c.offers[i].supplier_id !== q.supplier_id)
                invalid('offer_id', 'Only an unsent RFQ for the same supplier may be edited');
        }
        else {
            if (c.offers.some(o => o.supplier_id === q.supplier_id))
                invalid('supplier_id', 'Supplier already has an RFQ in this package');
            if (c.offers.length >= 50)
                invalid('offers', 'A sourcing case supports at most 50 suppliers');
            id(newID);
            c.offers.push(blankOffer(newID, q.supplier_id, actor, at));
            i = c.offers.length - 1;
        }
        Object.assign(c.offers[i], { due_date: q.due_date, rfq_terms: q.rfq_terms, supplier_snapshot: structuredClone(s) });
    }
    else {
        if (i < 0)
            throw missing();
        const o = c.offers[i];
        if (q.action !== 'no_bid')
            supplierGuard(suppliers.find(s => s.id === o.supplier_id));
        if (q.action === 'dispatch') {
            if (o.state !== 'draft')
                invalid('state', 'RFQ already sent');
            if (q.sent_date > today || q.sent_date < indiaDate(new Date(o.created_at)) || q.sent_date > o.due_date)
                invalid('sent_date', 'RFQ dispatch must be within creation/due date and not future');
            Object.assign(o, { state: 'sent', sent_date: q.sent_date, channel: q.channel, dispatch_reference: q.dispatch_reference });
        }
        else if (q.action === 'no_bid')
            Object.assign(o, { state: 'no_bid', no_bid_reason: q.reason });
        else if (q.action === 'quote') {
            if (!['sent', 'quoted', 'no_bid'].includes(o.state) || !o.sent_date)
                invalid('state', 'Record RFQ dispatch before entering a quotation');
            if (q.valid_until < today)
                invalid('valid_until', 'Quotation validity cannot be past');
            for (const field of ['quote_reference', 'evidence_reference', 'valid_until', 'pricing_mode', 'unit_price', 'discount', 'tax', 'freight', 'other', 'delivery_days', 'payment_terms', 'warranty', 'compliance_notes'] as const)
                Object.assign(o, { [field]: q[field] });
            if (q.line_total)
                o.line_total = q.line_total;
            else
                delete o.line_total;
            o.compliant = q.compliant!;
            total(o, c.source_snapshot.quantity);
            o.state = 'quoted';
            o.no_bid_reason = '';
        }
    }
    c.offers[i].updated_at = at;
    c.offers[i].updated_by_id = actor;
}
export function apply(base: Case | null, a: Actor, caps: Capabilities, raw: Command, src: Source, suppliers: Supplier[], members: Member[], proof: Proof | null, at: string, newCase: string, newOffer: string, project: string): Case {
    const q = validate(raw);
    accessGuard(a, caps, base, q.action);
    if (q.action !== 'reopen')
        guard(src);
    const version = base?.version ?? 0;
    if (q.version !== version)
        throw stale();
    const today = indiaDate(new Date(at));
    let c: Case;
    if (base) {
        c = structuredClone(base);
        if (!c.is_active)
            throw missing();
        if (!sameSource(c.source_snapshot, src))
            invalid('source', 'Released source changed; review current material evidence');
        if (c.state === 'released')
            invalid('state', 'Released LOI is immutable');
    }
    else {
        if (q.action !== 'configure')
            invalid('approver_id', 'Configure an independent sourcing approver first');
        id(newCase);
        id(project);
        c = { id: newCase, project_id: project, package_id: src.package_id, source_snapshot: structuredClone(src), approver_id: '', version: 0, state: 'draft', offers: [], selected_offer_id: '', selection_reason: '', loi_terms: '', proposed_by_id: '', approved_by_id: '', approved_at: null, decision_snapshot: null, loi_reference: '', loi_sent_date: '', loi_channel: '', loi_dispatch_reference: '', released_at: null, is_active: true, created_at: at, created_by_id: a.id, updated_at: '0001-01-01T00:00:00Z', updated_by_id: '' };
    }
    const member = (id: string) => members.some(m => m.id === id);
    if (q.action === 'configure') {
        if (c.state !== 'draft')
            invalid('state', 'Return to draft before changing the approver');
        if (!member(q.approver_id))
            invalid('approver_id', 'Select an active scoped sourcing approver');
        c.approver_id = q.approver_id;
    }
    else if (['rfq', 'dispatch', 'no_bid', 'quote'].includes(q.action)) {
        if (c.state !== 'draft')
            invalid('state', 'Return to draft before changing RFQs or quotations');
        offerCommand(c, q, suppliers, a.id, at, newOffer);
    }
    else if (q.action === 'propose') {
        if (c.state !== 'draft')
            invalid('state', 'Only draft sourcing may be submitted');
        if (c.offers.some(o => !['quoted', 'no_bid'].includes(o.state)))
            invalid('offers', 'Record a quotation or reasoned no-bid for every RFQ');
        if (a.id === c.approver_id)
            invalid('approver_id', 'Proposer and approver must differ');
        if (!member(c.approver_id))
            invalid('approver_id', 'Approver no longer has active project approval access');
        c.selected_offer_id = q.offer_id;
        const o = selected(c, today), s = suppliers.find(s => s.id === o.supplier_id);
        supplierGuard(s);
        proofGuard(c, o, proof);
        o.supplier_snapshot = structuredClone(s);
        o.updated_at = at;
        o.updated_by_id = a.id;
        const comparison = comparisons(c, today);
        c.selection_reason = q.reason;
        c.loi_terms = q.loi_terms;
        c.proposed_by_id = a.id;
        c.state = 'pending';
        c.decision_snapshot = { source: structuredClone(c.source_snapshot), offer: structuredClone(o), comparison, terms: q.loi_terms, reason: q.reason, financial_reference: proof.reference + ':' + proof.fingerprint };
    }
    else if (q.action === 'decide') {
        if (!member(a.id))
            throw forbidden();
        const o = selected(c, today), s = suppliers.find(s => s.id === o.supplier_id);
        supplierGuard(s);
        if (s.version !== o.supplier_snapshot.version)
            invalid('supplier_id', 'Supplier changed after submission; return to draft');
        proofGuard(c, o, proof);
        if (c.decision_snapshot?.financial_reference !== proof.reference + ':' + proof.fingerprint)
            invalid('financial_approval', 'Financial evidence changed after proposal; return to draft');
        if (q.approve)
            Object.assign(c, { state: 'approved', approved_by_id: a.id, approved_at: at });
        else
            Object.assign(c, { state: 'draft', approved_by_id: '', approved_at: null, decision_snapshot: null });
    }
    else if (q.action === 'reopen') {
        if (!['pending', 'approved'].includes(c.state))
            invalid('state', 'Only pending or approved sourcing may return to draft');
        Object.assign(c, { state: 'draft', approved_by_id: '', approved_at: null, proposed_by_id: '', decision_snapshot: null });
    }
    else if (q.action === 'release') {
        if (c.state !== 'approved' || !c.approved_at || !c.approved_by_id || c.approved_by_id === c.proposed_by_id)
            invalid('state', 'Independent approval is required before LOI release');
        const o = selected(c, today), s = suppliers.find(s => s.id === o.supplier_id);
        supplierGuard(s, true);
        if (s.version !== o.supplier_snapshot.version)
            invalid('supplier_id', 'Supplier changed after selection; return for new approval');
        if (!member(c.approver_id))
            invalid('approver_id', 'Approver access changed; return for new approval');
        proofGuard(c, o, proof);
        if (c.decision_snapshot?.financial_reference !== proof.reference + ':' + proof.fingerprint)
            invalid('financial_approval', 'Financial evidence changed; return for new approval');
        if (q.sent_date < indiaDate(new Date(c.approved_at)) || q.sent_date > today)
            invalid('sent_date', 'LOI dispatch must be on or after approval and not future');
        Object.assign(c, { loi_reference: 'LOI-' + c.id.replaceAll('-', '').toUpperCase(), loi_sent_date: q.sent_date, loi_channel: q.channel, loi_dispatch_reference: q.dispatch_reference, released_at: at, state: 'released' });
    }
    c.version = version + 1;
    c.updated_at = at;
    c.updated_by_id = a.id;
    return c;
}
export function released(c: Case) { if (!c.is_active || c.state !== 'released')
    return null; if (!c.decision_snapshot || !c.released_at || !c.loi_reference || !c.decision_snapshot.financial_reference || !c.approved_by_id || !c.approved_at || c.approved_by_id === c.proposed_by_id || c.approved_by_id !== c.approver_id || !c.decision_snapshot.offer.supplier_id)
    invalid('loi', 'Released commercial evidence is incomplete'); return { case_id: c.id, project_id: c.project_id, package_id: c.package_id, version: c.version, source: structuredClone(c.decision_snapshot.source), loi_terms: c.decision_snapshot.terms, loi_reference: c.loi_reference, offer: structuredClone(c.decision_snapshot.offer), total: total(c.decision_snapshot.offer, c.decision_snapshot.source.quantity), financial_reference: c.decision_snapshot.financial_reference, released_at: c.released_at }; }
export function currentReleased(c: Case, current: Source, supplier: Supplier | undefined, proof: Proof | null, today: string) { const fact = released(c); if (!fact)
    return null; guard(current); if (!sameSource(fact.source, current))
    invalid('source', 'Released LOI no longer matches the current item demand'); supplierGuard(supplier, true); const offer = selected(c, today); if (!isDeepStrictEqual(offer, fact.offer))
    invalid('loi', 'Released selection no longer matches its approved snapshot'); proofGuard(c, fact.offer, proof); if (fact.financial_reference !== proof.reference + ':' + proof.fingerprint)
    invalid('financial_approval', 'Financial approval changed after LOI release'); return fact; }
