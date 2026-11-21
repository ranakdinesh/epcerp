import { createHash } from "node:crypto";
import { AppError } from "../../errors";
import { goJSON } from "../catalogue/serialization";
import { byteLength, fieldError, goObject, integer, invalid, list, text, boolean } from "../catalogue/validation";
import { trim } from "../shared/text";
import { vendorID } from "../vendors/domain";
export type Evidence = {
    id: string;
    doc_type: string;
    sha256: string;
    version: number;
    previewable: boolean;
    approval_known: boolean;
    approval: string;
    approval_comment: string;
};
export type Snapshot = {
    id: string;
    version: number;
    active: boolean;
    gst_registered: boolean;
    documents: Evidence[];
};
export type Decision = {
    document_id: string;
    decision: string;
    comment: string;
};
export type Review = {
    supplier_id: string;
    version: number;
    fingerprint: string;
    decisions: Decision[] | null;
    reason: string;
    reviewed_at: string;
    reviewed_by_id: string | null;
};
export const staleReview = () => new AppError(409, "Supplier evidence or review changed; refresh and review again");
export function fingerprint(s: Snapshot) { return createHash('sha256').update(goJSON({ ID: s.id, Version: s.version, Active: s.active, GSTRegistered: s.gst_registered, Documents: [...s.documents].sort((a, b) => a.id < b.id ? -1 : a.id > b.id ? 1 : 0).map(d => ({ ID: d.id, DocType: d.doc_type, SHA256: d.sha256, Version: d.version, Previewable: d.previewable })) })).digest('hex'); }
export function parseReview(value: unknown) { const r = goObject(value, ["version", "fingerprint", "decisions", "reason"]); return { version: integer(r.version), fingerprint: text(r.fingerprint), reason: text(r.reason), decisions: list(r.decisions).map(v => { const d = goObject(v, ["document_id", "decision", "comment"]); return { document_id: text(d.document_id), decision: text(d.decision), comment: text(d.comment) }; }) }; }
export function validateDecisions(s: Snapshot, decisions: Decision[]) { const docs = new Map(s.documents.map(d => [d.id, d])), seen = new Set<string>(); for (const d of decisions) {
    const document = docs.get(d.document_id);
    if (!document || seen.has(d.document_id))
        invalid("Review only active documents without duplicates");
    seen.add(d.document_id);
    if (!["accept", "reject"].includes(d.decision))
        fieldError('decision_' + d.document_id, "Choose accept or reject for every document");
    if (!trim(d.comment) || byteLength(d.comment) > 2000)
        fieldError('comment_' + d.document_id, "A review comment is required for every document");
    if (d.decision === 'accept' && !document!.previewable)
        fieldError('decision_' + d.document_id, "This file cannot be previewed. Choose Replace on this document and upload a readable PDF or image.");
} if (!seen.size)
    invalid("Choose a document to review"); }
export function validateReview(s: Snapshot, c: ReturnType<typeof parseReview>) { if (!s.active)
    invalid("Inactive or suspended supplier cannot be verified"); if (!trim(c.reason) || byteLength(c.reason) > 2000)
    fieldError('reason', "Review reason is required (maximum 2000 bytes)"); validateDecisions(s, c.decisions); }
export function evaluate(s: Snapshot, previous: Review | null, hash = fingerprint(s)) {
    const effective: Review = previous ? { ...previous, fingerprint: hash, decisions: null } : { supplier_id: "", version: 0, fingerprint: hash, decisions: null, reason: "", reviewed_at: "0001-01-01T00:00:00Z", reviewed_by_id: null };
    for (const d of s.documents) {
        if (d.approval_known) {
            if (['accept', 'reject'].includes(d.approval))
                (effective.decisions ??= []).push({ document_id: d.id, decision: d.approval, comment: d.approval_comment });
        }
        else if (previous?.fingerprint === hash) {
            for (const decision of previous.decisions ?? [])
                if (decision.document_id === d.id)
                    (effective.decisions ??= []).push(decision);
        }
    }
    const review = previous || (effective.decisions?.length ?? 0) > 0 ? effective : undefined, found = new Set(s.documents.map(d => d.doc_type)), missing = ['pan', 'cheque', ...(s.gst_registered ? ['gst'] : [])].filter(t => !found.has(t));
    const result = { state: 'unverified', message: 'Finance must verify all active documents before a purchase order', missing_types: missing, can_invite_quotation: s.active, can_generate_purchase_order: false, fingerprint: hash, ...(review ? { review } : {}) };
    if (!s.active) {
        result.state = 'blocked';
        result.message = 'Supplier is inactive or suspended';
        return result;
    }
    if (missing.length) {
        result.state = 'blocked';
        result.message = 'Required evidence missing: ' + missing.join(', ');
        return result;
    }
    if (s.documents.some(d => !d.previewable)) {
        result.state = 'blocked';
        result.message = 'Supplier document content is missing or unreadable; restore or replace the file before verification';
        return result;
    }
    if (!review || review.fingerprint !== hash)
        return result;
    try {
        validateDecisions(s, review.decisions ?? []);
    }
    catch {
        return result;
    }
    if (review.decisions?.some(d => d.decision === 'reject')) {
        result.state = 'rejected';
        result.message = 'Finance rejected supplier evidence; resolve the document review';
        return result;
    }
    if (review.decisions?.length !== s.documents.length) {
        result.message = 'Finance document review in progress';
        return result;
    }
    result.state = 'verified';
    result.message = 'All active supplier documents verified by Finance';
    result.can_generate_purchase_order = true;
    return result;
}
export function parseAssignment(value: unknown) { const r = goObject(value, ['user_id', 'version', 'is_active', 'reason']); if (r.is_active == null)
    fieldError('is_active', 'Assignment status is required'); return { user_id: text(r.user_id), version: integer(r.version), is_active: boolean(r.is_active), reason: text(r.reason) }; }
export function validateAssignment(r: ReturnType<typeof parseAssignment>) { if (!vendorID.test(r.user_id))
    fieldError('user_id', 'Choose an active Finance reviewer'); if (r.version < 0)
    throw staleReview(); if (!trim(r.reason) || byteLength(r.reason) > 2000)
    fieldError('reason', 'Assignment reason is required (maximum 2000 bytes)'); }
