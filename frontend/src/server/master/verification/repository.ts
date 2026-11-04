import "server-only";
import { randomUUID } from "node:crypto";
import type { DB } from "../../db";
import { recordMasterEvent } from "../../events/journal";
import { supplierReviewNotice } from "../../inbox/supplier-review";
import { query } from "../catalogue/database";
import { VendorRepository } from "../vendors/repository";
import { trim } from "../shared/text";
import { open, readStoredObject, inspect } from "../documents/files";
import { MasterValidationError } from "../shared/validation";
import type { Document } from "../documents/domain";
import type { Review, Snapshot, Decision } from "./domain";
import { missingVendor } from "../vendors/domain";
export class VerificationRepository {
    constructor(private db: DB) { }
    async snapshot(id: string): Promise<Snapshot> {
        const supplier = await new VendorRepository(this.db).get(id);
        if (!supplier)
            throw missingVendor();
        const documents = [];
        for (const d of (await query<Document>(this.db, 'VendorDocumentsActiveEvidence', { supplier_id: id })).rows) {
            let previewable = false, missing = false, encrypted = d.encrypted_content;
            if (d.object_key)
                try {
                    encrypted = await readStoredObject(d.object_bucket!, d.object_key);
                }
                catch (e) {
                    if (['NoSuchKey', 'NoSuchBucket', 'NotFound'].includes((e as Error).name) || (e as {
                        $metadata?: {
                            httpStatusCode: number;
                        };
                    }).$metadata?.httpStatusCode === 404)
                        missing = true;
                    else
                        throw e;
                }
            if (!missing) {
                const data = open(encrypted ?? Buffer.alloc(0), d.sha256);
                try {
                    await inspect(data);
                    previewable = true;
                }
                catch (e) {
                    if (!(e instanceof MasterValidationError))
                        throw e;
                }
            }
            const approval = (await query(this.db, 'DocumentApprovalState', { document_id: d.id, document_version: d.version, sha256: d.sha256 })).rows[0];
            documents.push({ id: d.id, doc_type: d.doc_type, sha256: d.sha256, version: d.version, previewable, approval_known: !!approval, approval: approval?.state ?? '', approval_comment: approval?.comment ?? '' });
        }
        return { id, version: supplier.version, active: supplier.is_active && supplier.data.qualification !== 'suspended', gst_registered: !!trim(typeof supplier.data.gst_number === 'string' ? supplier.data.gst_number : ''), documents };
    }
    async review(supplier_id: string): Promise<Review | null> { const row = (await query(this.db, 'VendorVerificationGetReview', { supplier_id })).rows[0]; return row ? { supplier_id: row.supplier_id, version: row.version, fingerprint: row.fingerprint, decisions: row.decisions, reason: row.reason, reviewed_at: row.updated_at, reviewed_by_id: row.updated_by_id } : null; }
    async saveReview(supplier_id: string, version: number, fingerprint: string, decisions: Decision[], reason: string, actor: string) {
        const saved = (await query(this.db, 'VendorVerificationSaveReview', { supplier_id, version, fingerprint, decisions: JSON.stringify(decisions), reason, actor })).rows[0];
        const snapshot = { supplier_id, version: saved.version, fingerprint: saved.fingerprint, decisions: saved.decisions, reason: saved.reason, reviewed_at: saved.updated_at, reviewed_by_id: saved.updated_by_id }, id = randomUUID();
        await query(this.db, 'VendorVerificationRecordChange', { id, supplier_id, version, snapshot, actor });
        await recordMasterEvent(this.db, 'supplier_verification', 'supplier_verification', supplier_id, version, 'reviewed', actor, id, saved.updated_at);
        return snapshot as Review;
    }
    async lockSupplier(supplier_id: string) { if (!(await query(this.db, 'VendorReviewLockSupplier', { supplier_id })).rows[0])
        throw missingVendor(); }
    async assignment(supplier_id: string) { return (await query(this.db, 'VendorReviewAssignmentGet', { supplier_id })).rows[0] ?? null; }
    async saveAssignment(supplier_id: string, input: {
        user_id: string;
        is_active: boolean;
        reason: string;
    }, current: Awaited<ReturnType<VerificationRepository['assignment']>>, actor: string) {
        const id = current?.id ?? randomUUID(), version = (current?.version ?? 0) + 1;
        await query(this.db, 'VendorReviewAssignmentSave', { id, supplier_id, user_id: input.user_id, version, active: input.is_active, actor });
        if (input.is_active)
            await query(this.db, 'DocumentTasksReassign', { supplier_id, user_id: input.user_id, actor });
        const saved = (await this.assignment(supplier_id))!, { username: _, ...snapshot } = saved, event = randomUUID();
        void _;
        await query(this.db, 'VendorReviewAssignmentJournal', { id: event, supplier_id, version, reason: input.reason, snapshot, actor });
        await recordMasterEvent(this.db, 'supplier_review_assignments', 'supplier_review_assignment', id, version, input.is_active ? 'assigned' : 'revoked', actor, event, saved.updated_at);
        if (input.is_active)
            await supplierReviewNotice(this.db, event, input.user_id, supplier_id);
        return assignmentView(saved);
    }
}
export function assignmentView(row: Record<string, unknown> | null) { return row ? { id: row.id, supplier_id: row.supplier_id, user_id: row.user_id, reviewer_name: row.username, version: row.version, is_active: row.is_active, updated_at: row.updated_at } : null; }
